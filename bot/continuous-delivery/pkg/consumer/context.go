package consumer

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v29/github"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/bot/continuous-delivery/pkg/config"
)

type eventContext struct {
	Owner             string
	Repo              string
	Commit            string
	Rule              *config.BuildRule
	PullRequestNumber int
	Changed           []string
}

type dnsControlContext struct {
	*eventContext
	Rule *config.DNSControlRule
}

func NewEventContextFromPushEvent(event *github.PushEvent) *eventContext {
	commit := event.GetAfter()
	if commit == "0000000000000000000000000000000000000000" {
		commit = event.GetBefore()
	}
	s := strings.SplitN(event.Repo.GetFullName(), "/", 2)
	ctx := &eventContext{
		Owner:  s[0],
		Repo:   s[1],
		Commit: commit,
	}

	return ctx
}

func NewEventContextFromPullRequest(event *github.PullRequestEvent) *eventContext {
	s := strings.SplitN(event.Repo.GetFullName(), "/", 2)
	ctx := &eventContext{
		Owner:             s[0],
		Repo:              s[1],
		Commit:            event.PullRequest.Head.GetSHA(),
		PullRequestNumber: event.PullRequest.GetNumber(),
	}

	return ctx
}

func (c *eventContext) FetchRuleFile(hClient *http.Client, path string) (string, error) {
	log.Printf("Fetch rule file via api: %s/%s %s %s", c.Owner, c.Repo, c.Commit, path)
	client := github.NewClient(hClient)
	t, _, err := client.Git.GetTree(context.Background(), c.Owner, c.Repo, c.Commit, true)
	if err != nil {
		return "", xerrors.Errorf(": %v", err)
	}

	fileSHA := ""
	for _, v := range t.Entries {
		if v.GetPath() != path {
			continue
		}
		fileSHA = v.GetSHA()
		break
	}

	if fileSHA == "" {
		return "", xerrors.New("run rule file is not found")
	}

	b, _, err := client.Git.GetBlob(context.Background(), c.Owner, c.Repo, fileSHA)
	if err != nil {
		return "", xerrors.Errorf(": %v", err)
	}
	buf, err := base64.StdEncoding.DecodeString(b.GetContent())
	if err != nil {
		return "", xerrors.Errorf(": %v", err)
	}

	return string(buf), nil
}

func (c *eventContext) CloneURL() string {
	return fmt.Sprintf("https://github.com/%s/%s.git", c.Owner, c.Repo)
}
