// Package server implements the Connect API of the issue tracker.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/issue"
	"go.f110.dev/mono/go/issue/api"
	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/logger"
)

// Repositories looks up the repositories of Build to build the URLs of the commits and the pull requests.
type Repositories interface {
	// URLs returns the URLs of the repositories of ids. The repository which is not found is not contained.
	URLs(ctx context.Context, ids []int32) (map[int32]string, error)
}

// IssueHandler implements api.IssueHandler on top of issue.Tracker.
type IssueHandler struct {
	tracker      *issue.Tracker
	repositories Repositories
}

var _ api.IssueHandler = (*IssueHandler)(nil)

// NewIssueHandler returns the handler. If repositories is nil, the commits and the pull requests have no URL.
func NewIssueHandler(t *issue.Tracker, repositories Repositories) *IssueHandler {
	return &IssueHandler{tracker: t, repositories: repositories}
}

const (
	defaultPageSize = 100
	maxPageSize     = 500
)

// pageToken is the content of page_token of ListTickets. filter and order_by are kept to reject the token which is used
// with the other request.
type pageToken struct {
	Filter  string `json:"filter"`
	OrderBy string `json:"order_by"`
	Offset  int    `json:"offset"`
}

func (p pageToken) encode() string {
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodePageToken(s string) (pageToken, error) {
	var p pageToken
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return p, xerrors.WithStack(err)
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, xerrors.WithStack(err)
	}
	return p, nil
}

func (h *IssueHandler) ListTickets(ctx context.Context, req *connect.Request[api.RequestListTickets]) (*connect.Response[api.ResponseListTickets], error) {
	size := int(req.Msg.GetPageSize())
	switch {
	case size < 0:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page_size must not be negative"))
	case size == 0:
		size = defaultPageSize
	case size > maxPageSize:
		size = maxPageSize
	}
	page := pageToken{Filter: req.Msg.GetFilter(), OrderBy: req.Msg.GetOrderBy()}
	if req.Msg.GetPageToken() != "" {
		p, err := decodePageToken(req.Msg.GetPageToken())
		if err != nil || p.Filter != page.Filter || p.OrderBy != page.OrderBy || p.Offset < 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid page_token"))
		}
		page = p
	}

	tickets, err := h.tracker.Find(ctx, page.Filter, page.OrderBy, page.Offset, size+1)
	if err != nil {
		return nil, toConnectError(err)
	}
	var next string
	if len(tickets) > size {
		tickets = tickets[:size]
		next = pageToken{Filter: page.Filter, OrderBy: page.OrderBy, Offset: page.Offset + size}.encode()
	}
	return connect.NewResponse(api.ResponseListTickets_builder{Tickets: enumerable.Map(tickets, toTicket), NextPageToken: &next}.Build()), nil
}

func (h *IssueHandler) GetTicket(ctx context.Context, req *connect.Request[api.RequestGetTicket]) (*connect.Response[api.ResponseGetTicket], error) {
	ticket, err := h.tracker.Ticket(ctx, req.Msg.GetNumber())
	if err != nil {
		return nil, toConnectError(err)
	}
	relations, err := ticket.Relations(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	comments, err := ticket.Comments(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	backlinks, err := ticket.Backlinks(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	commits, err := ticket.CommitLinks(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	pullRequests, err := ticket.PullRequestLinks(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	var repositoryIDs []int32
	for _, v := range commits {
		repositoryIDs = append(repositoryIDs, v.RepositoryId)
	}
	for _, v := range pullRequests {
		repositoryIDs = append(repositoryIDs, v.RepositoryId)
	}
	urls := h.repositoryURLs(ctx, repositoryIDs)
	return connect.NewResponse(api.ResponseGetTicket_builder{
		Ticket: toTicket(ticket),
		Relations: enumerable.Map(relations, func(r *database.Relation) *api.RelatedTicket {
			return toRelatedTicket(r, ticket.Id)
		}),
		Comments:  enumerable.Map(comments, toComment),
		Backlinks: enumerable.Map(backlinks, toBacklink),
		Commits: enumerable.Map(commits, func(c *database.CommitLink) *api.CommitLink {
			return toCommitLink(c, urls[c.RepositoryId])
		}),
		PullRequests: enumerable.Map(pullRequests, func(p *database.PullRequestLink) *api.PullRequestLink {
			return toPullRequestLink(p, urls[p.RepositoryId])
		}),
	}.Build()), nil
}

// repositoryURLs returns the URLs of the repositories. The ticket is shown without the URLs if Build is not available.
func (h *IssueHandler) repositoryURLs(ctx context.Context, ids []int32) map[int32]string {
	if h.repositories == nil || len(ids) == 0 {
		return nil
	}
	urls, err := h.repositories.URLs(ctx, ids)
	if err != nil {
		logger.Log.Warn("Failed to get the URLs of the repositories", logger.E(err))
		return nil
	}
	return urls
}

func (h *IssueHandler) CreateTicket(ctx context.Context, req *connect.Request[api.RequestCreateTicket]) (*connect.Response[api.ResponseCreateTicket], error) {
	var componentID, assigneeID *int32
	if req.Msg.HasComponentId() {
		componentID = new(req.Msg.GetComponentId())
	}
	if req.Msg.HasAssigneeId() {
		assigneeID = new(req.Msg.GetAssigneeId())
	}
	ticket, err := h.tracker.Create(ctx, req.Msg.GetTitle(), req.Msg.GetBody(), database.TicketType(req.Msg.GetType()), database.Priority(req.Msg.GetPriority()), componentID, assigneeID)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseCreateTicket_builder{Ticket: toTicket(ticket)}.Build()), nil
}

func (h *IssueHandler) UpdateTicket(ctx context.Context, req *connect.Request[api.RequestUpdateTicket]) (*connect.Response[api.ResponseUpdateTicket], error) {
	var title, body *string
	var status *database.TicketStatus
	var ticketType *database.TicketType
	var priority *database.Priority
	var componentID, assigneeID *int32
	if req.Msg.HasTitle() {
		title = new(req.Msg.GetTitle())
	}
	if req.Msg.HasBody() {
		body = new(req.Msg.GetBody())
	}
	if req.Msg.HasStatus() {
		s, err := toDBStatus(req.Msg.GetStatus())
		if err != nil {
			return nil, err
		}
		status = &s
	}
	if req.Msg.HasType() {
		ticketType = new(database.TicketType(req.Msg.GetType()))
	}
	if req.Msg.HasPriority() {
		priority = new(database.Priority(req.Msg.GetPriority()))
	}
	if req.Msg.HasComponentId() {
		componentID = new(req.Msg.GetComponentId())
	}
	if req.Msg.HasAssigneeId() {
		assigneeID = new(req.Msg.GetAssigneeId())
	}
	ticket, err := h.tracker.Ticket(ctx, req.Msg.GetNumber())
	if err != nil {
		return nil, toConnectError(err)
	}
	if err := ticket.Update(ctx, title, body, status, ticketType, priority, componentID, assigneeID); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseUpdateTicket_builder{Ticket: toTicket(ticket)}.Build()), nil
}

func (h *IssueHandler) AddRelation(ctx context.Context, req *connect.Request[api.RequestAddRelation]) (*connect.Response[api.ResponseAddRelation], error) {
	rel := req.Msg.GetRelation()
	relationType, err := toDBRelationType(rel.GetType())
	if err != nil {
		return nil, err
	}
	ticket, err := h.tracker.Ticket(ctx, rel.GetFrom())
	if err != nil {
		return nil, toConnectError(err)
	}
	r, err := ticket.Relate(ctx, relationType, rel.GetTo())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseAddRelation_builder{Relation: toRelation(r)}.Build()), nil
}

func (h *IssueHandler) RemoveRelation(ctx context.Context, req *connect.Request[api.RequestRemoveRelation]) (*connect.Response[api.ResponseRemoveRelation], error) {
	rel := req.Msg.GetRelation()
	relationType, err := toDBRelationType(rel.GetType())
	if err != nil {
		return nil, err
	}
	ticket, err := h.tracker.Ticket(ctx, rel.GetFrom())
	if err != nil {
		return nil, toConnectError(err)
	}
	if err := ticket.Unrelate(ctx, relationType, rel.GetTo()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&api.ResponseRemoveRelation{}), nil
}

func (h *IssueHandler) AddComment(ctx context.Context, req *connect.Request[api.RequestAddComment]) (*connect.Response[api.ResponseAddComment], error) {
	ticket, err := h.tracker.Ticket(ctx, req.Msg.GetTicket())
	if err != nil {
		return nil, toConnectError(err)
	}
	c, err := ticket.AddComment(ctx, req.Msg.GetBody())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseAddComment_builder{Comment: toComment(c)}.Build()), nil
}

func (h *IssueHandler) EditComment(ctx context.Context, req *connect.Request[api.RequestEditComment]) (*connect.Response[api.ResponseEditComment], error) {
	c, err := h.tracker.Comment(ctx, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}
	if err := c.Edit(ctx, req.Msg.GetBody()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseEditComment_builder{Comment: toComment(c)}.Build()), nil
}

func (h *IssueHandler) DeleteComment(ctx context.Context, req *connect.Request[api.RequestDeleteComment]) (*connect.Response[api.ResponseDeleteComment], error) {
	c, err := h.tracker.Comment(ctx, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}
	if err := c.Delete(ctx); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&api.ResponseDeleteComment{}), nil
}

func (h *IssueHandler) ListActivities(ctx context.Context, req *connect.Request[api.RequestListActivities]) (*connect.Response[api.ResponseListActivities], error) {
	ticket, err := h.tracker.Ticket(ctx, req.Msg.GetTicket())
	if err != nil {
		return nil, toConnectError(err)
	}
	activities, err := ticket.Activities(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseListActivities_builder{Activities: enumerable.Map(activities, toActivity)}.Build()), nil
}

func (h *IssueHandler) ListTicketRevisions(ctx context.Context, req *connect.Request[api.RequestListTicketRevisions]) (*connect.Response[api.ResponseListTicketRevisions], error) {
	ticket, err := h.tracker.Ticket(ctx, req.Msg.GetTicket())
	if err != nil {
		return nil, toConnectError(err)
	}
	revs, err := ticket.Revisions(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseListTicketRevisions_builder{Revisions: enumerable.Map(revs, toRevision)}.Build()), nil
}

func (h *IssueHandler) ListCommentRevisions(ctx context.Context, req *connect.Request[api.RequestListCommentRevisions]) (*connect.Response[api.ResponseListCommentRevisions], error) {
	c, err := h.tracker.Comment(ctx, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}
	revs, err := c.Revisions(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseListCommentRevisions_builder{Revisions: enumerable.Map(revs, toRevision)}.Build()), nil
}

func (h *IssueHandler) ListComponents(ctx context.Context, _ *connect.Request[api.RequestListComponents]) (*connect.Response[api.ResponseListComponents], error) {
	components, err := h.tracker.Components(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseListComponents_builder{Components: enumerable.Map(components, toComponent)}.Build()), nil
}

func (h *IssueHandler) SaveComponent(ctx context.Context, req *connect.Request[api.RequestSaveComponent]) (*connect.Response[api.ResponseSaveComponent], error) {
	c, err := h.tracker.SaveComponent(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseSaveComponent_builder{Component: toComponent(c)}.Build()), nil
}

func (h *IssueHandler) ListTemplates(ctx context.Context, _ *connect.Request[api.RequestListTemplates]) (*connect.Response[api.ResponseListTemplates], error) {
	templates, err := h.tracker.Templates(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseListTemplates_builder{Templates: enumerable.Map(templates, toTemplate)}.Build()), nil
}

func (h *IssueHandler) SaveTemplate(ctx context.Context, req *connect.Request[api.RequestSaveTemplate]) (*connect.Response[api.ResponseSaveTemplate], error) {
	var componentID *int32
	if req.Msg.GetComponentId() != 0 {
		componentID = new(req.Msg.GetComponentId())
	}
	tmpl, err := h.tracker.SaveTemplate(ctx, req.Msg.GetId(), req.Msg.GetTitle(), req.Msg.GetBody(), componentID)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseSaveTemplate_builder{Template: toTemplate(tmpl)}.Build()), nil
}

func (h *IssueHandler) DeleteTemplate(ctx context.Context, req *connect.Request[api.RequestDeleteTemplate]) (*connect.Response[api.ResponseDeleteTemplate], error) {
	if err := h.tracker.DeleteTemplate(ctx, req.Msg.GetId()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&api.ResponseDeleteTemplate{}), nil
}

func (h *IssueHandler) ListActors(ctx context.Context, _ *connect.Request[api.RequestListActors]) (*connect.Response[api.ResponseListActors], error) {
	actors, err := h.tracker.Actors(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseListActors_builder{Actors: enumerable.Map(actors, toActor)}.Build()), nil
}

func (h *IssueHandler) WhoAmI(ctx context.Context, _ *connect.Request[api.RequestWhoAmI]) (*connect.Response[api.ResponseWhoAmI], error) {
	actor, err := h.tracker.Me(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseWhoAmI_builder{Actor: toActor(actor)}.Build()), nil
}

func toConnectError(err error) error {
	switch {
	case errors.Is(err, issue.ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, issue.ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, issue.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, issue.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	}
	logger.Log.Error("Failed to handle the request", logger.E(err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func toDBStatus(s api.TicketStatus) (database.TicketStatus, error) {
	if s == api.TicketStatus_TICKET_STATUS_UNSPECIFIED {
		return 0, connect.NewError(connect.CodeInvalidArgument, errors.New("status is unspecified"))
	}
	return database.TicketStatus(s - 1), nil
}

func toDBRelationType(t api.RelationType) (database.RelationType, error) {
	if t == api.RelationType_RELATION_TYPE_UNSPECIFIED {
		return 0, connect.NewError(connect.CodeInvalidArgument, errors.New("relation type is unspecified"))
	}
	return database.RelationType(t - 1), nil
}

func toTicket(t *issue.Ticket) *api.Ticket {
	b := api.Ticket_builder{
		Number:         new(t.Id),
		Title:          new(t.Title),
		Body:           new(t.Body),
		Status:         new(api.TicketStatus(t.Status + 1)),
		Priority:       new(api.Priority(t.Priority)),
		Type:           new(api.TicketType(t.Type)),
		ComponentId:    t.ComponentId,
		AssigneeId:     t.AssigneeId,
		ReporterId:     new(t.ReporterId),
		CreatedAt:      timestamppb.New(t.CreatedAt),
		LastActivityAt: timestamppb.New(t.LastActivityAt),
	}
	if t.UpdatedAt != nil {
		b.UpdatedAt = timestamppb.New(*t.UpdatedAt)
	}
	return b.Build()
}

func toActor(a *database.Actor) *api.Actor {
	return api.Actor_builder{
		Id:      new(a.Id),
		Kind:    new(api.ActorKind(a.Kind + 1)),
		Name:    new(a.Name),
		OwnerId: a.OwnerId,
	}.Build()
}

func toComponent(c *database.Component) *api.Component {
	return api.Component_builder{Id: new(c.Id), Name: new(c.Name)}.Build()
}

func toTemplate(t *database.TicketTemplate) *api.Template {
	return api.Template_builder{Id: new(t.Id), Title: new(t.Title), Body: new(t.Body), ComponentId: t.ComponentId}.Build()
}

func toRelation(r *database.Relation) *api.Relation {
	return api.Relation_builder{
		From: new(r.FromId),
		Type: new(api.RelationType(r.RelationType + 1)),
		To:   new(r.ToId),
	}.Build()
}

// toRelatedTicket converts the relation of the ticket. number is the ticket which the relation is viewed from.
func toRelatedTicket(r *database.Relation, number int32) *api.RelatedTicket {
	b := api.RelatedTicket_builder{Relation: toRelation(r)}
	other := r.To
	if r.ToId == number {
		other = r.From
	}
	if other != nil {
		b.Title = new(other.Title)
	}
	return b.Build()
}

func toBacklink(m *database.Mention) *api.Backlink {
	b := api.Backlink_builder{Ticket: new(m.FromId), CreatedAt: timestamppb.New(m.CreatedAt)}
	if m.From != nil {
		b.Title = new(m.From.Title)
	}
	return b.Build()
}

// toCommitLink converts the commit. repositoryURL is the URL of the repository, and the commit has no URL if it is empty.
func toCommitLink(c *database.CommitLink, repositoryURL string) *api.CommitLink {
	b := api.CommitLink_builder{
		Sha:         new(c.Sha),
		ChangeId:    c.ChangeId,
		Subject:     new(c.Subject),
		Author:      new(c.Author),
		Trailer:     new(api.TrailerKind(c.Trailer + 1)),
		CommittedAt: timestamppb.New(c.CommittedAt),
	}
	if repositoryURL != "" {
		b.Url = new(repositoryURL + "/commit/" + c.Sha)
	}
	return b.Build()
}

// toPullRequestLink converts the pull request. repositoryURL is the URL of the repository, and the pull request has no
// URL if it is empty.
func toPullRequestLink(p *database.PullRequestLink, repositoryURL string) *api.PullRequestLink {
	b := api.PullRequestLink_builder{
		Number:  new(p.Number),
		Title:   new(p.Title),
		Author:  new(p.Author),
		State:   new(api.PullRequestState(p.State + 1)),
		HeadSha: new(p.HeadSha),
	}
	if repositoryURL != "" {
		b.Url = new(fmt.Sprintf("%s/pull/%d", repositoryURL, p.Number))
	}
	return b.Build()
}

func toComment(c *issue.Comment) *api.Comment {
	b := api.Comment_builder{
		Id:        new(c.Id),
		Ticket:    new(c.TicketId),
		ActorId:   new(c.ActorId),
		Body:      new(c.Body),
		CreatedAt: timestamppb.New(c.CreatedAt),
	}
	if c.UpdatedAt != nil {
		b.UpdatedAt = timestamppb.New(*c.UpdatedAt)
	}
	return b.Build()
}

func toRevision(r *issue.Revision) *api.Revision {
	return api.Revision_builder{
		Id:        new(r.Id),
		Body:      new(r.Body),
		ActorId:   new(r.ActorId),
		CreatedAt: timestamppb.New(r.CreatedAt),
	}.Build()
}

func toActivity(a *database.Activity) *api.Activity {
	return api.Activity_builder{
		Id:        new(a.Id),
		Ticket:    new(a.TicketId),
		ActorId:   new(a.ActorId),
		Kind:      new(api.ActivityKind(a.Kind + 1)),
		FromValue: new(a.FromValue),
		ToValue:   new(a.ToValue),
		RefId:     a.RefId,
		CreatedAt: timestamppb.New(a.CreatedAt),
	}.Build()
}

// New returns the HTTP handler which serves the Connect API. The requests are authenticated by authenticate which puts
// the claims of heimdallr to the context (e.g. authproxy.Authenticate), and the human of the subject is the actor of the
// operations.
func New(tracker *issue.Tracker, repositories Repositories, authenticate func(http.Handler) http.Handler) http.Handler {
	path, handler := api.NewIssueHandler(NewIssueHandler(tracker, repositories))
	mux := http.NewServeMux()
	mux.Handle(path, authenticate(withHuman(handler)))
	return mux
}

func withHuman(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		if claims, ok := authproxy.ClaimsFrom(ctx); ok {
			ctx = issue.WithHuman(ctx, claims.Subject)
		}
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}
