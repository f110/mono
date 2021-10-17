package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

const (
	BaseURL   = "https://api.notion.com/v1"
	UserAgent = "go.f110.dev/notion-api v2"

	notionVersion = "2021-08-16"
)

var (
	ErrBadRequest       = errors.New("notion: bad request")
	ErrLimitExceeded    = errors.New("notion: limit exceeded")
	ErrUserNotFound     = errors.New("notion: user not found")
	ErrDatabaseNotFound = errors.New("notion: database not found")
	ErrPageNotFound     = errors.New("notion: page not found")
	ErrBlockNotFound    = errors.New("notion: block not found")
)

type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
}

func New(c *http.Client, baseURL string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed parse base URL: %v", err)
	}
	if u.Path != "/v1" {
		u.Path = "/v1"
	}

	return &Client{httpClient: c, baseURL: u}, nil
}

// GetUser can get a user.
// ref: https://developers.notion.com/reference/get-user
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/users/%s", userID), nil, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrUserNotFound
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	}

	obj := &User{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response: %v", err)
	}

	return obj, nil
}

// ListAllUsers can get all users.
// ref: https://developers.notion.com/reference/get-users
func (c *Client) ListAllUsers(ctx context.Context) ([]*User, error) {
	params := &url.Values{}
	params.Set("page_size", "100")

	users := make([]*User, 0)
	for {
		req, err := c.newRequest(ctx, http.MethodGet, "/users", params, nil)
		if err != nil {
			return nil, err
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest:
			res.Body.Close()
			return nil, ErrBadRequest
		}

		obj := &UserList{}
		if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
			res.Body.Close()
			return nil, fmt.Errorf("failed parse a response: %v", err)
		}
		users = append(users, obj.Results...)
		res.Body.Close()

		if !obj.HasMore {
			break
		}
		params.Set("start_cursor", obj.NextCursor)
	}

	return users, nil
}

// ListDatabases can get all databases.
// ref: https://developers.notion.com/reference/get-databases
func (c *Client) ListDatabases(ctx context.Context) ([]*Database, error) {
	params := &url.Values{}
	params.Set("page_size", "100")

	databases := make([]*Database, 0)
	for {
		req, err := c.newRequest(ctx, http.MethodGet, "/databases", params, nil)
		if err != nil {
			return nil, err
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest:
			res.Body.Close()
			return nil, ErrBadRequest
		case http.StatusTooManyRequests:
			res.Body.Close()
			return nil, ErrLimitExceeded
		}

		obj := &DatabaseList{}
		if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
			res.Body.Close()
			return nil, fmt.Errorf("failed parse a response: %v", err)
		}
		databases = append(databases, obj.Results...)
		res.Body.Close()

		if !obj.HasMore {
			break
		}
		params.Set("start_cursor", obj.NextCursor)
	}

	return databases, nil
}

// GetDatabase can get a database.
// ref: https://developers.notion.com/reference/get-database
func (c *Client) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/databases/%s", databaseID), nil, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrDatabaseNotFound
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &Database{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response: %v", err)
	}

	return obj, nil
}

// UpdateDatabase can update a database.
// ref: https://developers.notion.com/reference/update-a-database
func (c *Client) UpdateDatabase(ctx context.Context, db *Database) (*Database, error) {
	body := struct {
		Title      []*RichTextObject            `json:"title,omitempty"`
		Properties map[string]*PropertyMetadata `json:"properties"`
	}{
		Properties: db.Properties,
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return nil, fmt.Errorf("notion: failed to encode request body: %v", err)
	}
	req, err := c.newRequest(ctx, http.MethodPatch, fmt.Sprintf("/databases/%s", db.ID), nil, buf)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrDatabaseNotFound
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &Database{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("notion: failed parse a response: %v", err)
	}

	return obj, nil
}

// GetPages can get all pages which belongs to the database.
// ref: https://developers.notion.com/reference/post-database-query
func (c *Client) GetPages(ctx context.Context, databaseID string, filter *Filter, sorts []*Sort) ([]*Page, error) {
	data := &struct {
		Filter      *Filter `json:"filter,omitempty"`
		Sorts       []*Sort `json:"sorts,omitempty"`
		PageSize    int     `json:"page_size"`
		StartCursor string  `json:"start_cursor,omitempty"`
	}{
		Filter: filter, Sorts: sorts,
	}

	pages := make([]*Page, 0)
	for {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(data); err != nil {
			return nil, err
		}

		req, err := c.newRequest(ctx, http.MethodPost, fmt.Sprintf("/databases/%s/query", databaseID), nil, buf)
		if err != nil {
			return nil, err
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest:
			res.Body.Close()
			return nil, ErrBadRequest
		case http.StatusTooManyRequests:
			res.Body.Close()
			return nil, ErrLimitExceeded
		}

		obj := &PageList{}
		if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
			res.Body.Close()
			return nil, fmt.Errorf("failed parse a response: %v", err)
		}
		pages = append(pages, obj.Results...)
		res.Body.Close()

		if !obj.HasMore {
			break
		}
		data.StartCursor = obj.NextCursor
	}

	return pages, nil
}

// GetPage can get single page.
// ref: https://developers.notion.com/reference/get-page
func (c *Client) GetPage(ctx context.Context, pageID string) (*Page, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/pages/%s", pageID), nil, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrPageNotFound
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &Page{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response body: %v", err)
	}

	return obj, nil
}

// GetBlocks can get children block.
// ref: https://developers.notion.com/reference/get-block-children
func (c *Client) GetBlocks(ctx context.Context, pageID string) ([]*Block, error) {
	params := &url.Values{}
	params.Set("page_size", "100")

	blocks := make([]*Block, 0)
	for {
		req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/blocks/%s/children", pageID), params, nil)
		if err != nil {
			return nil, err
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest:
			res.Body.Close()
			return nil, ErrBadRequest
		case http.StatusTooManyRequests:
			res.Body.Close()
			return nil, ErrLimitExceeded
		}

		obj := &BlockList{}
		if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
			res.Body.Close()
			return nil, fmt.Errorf("failed parse a response: %v", err)
		}
		blocks = append(blocks, obj.Results...)
		res.Body.Close()

		if !obj.HasMore {
			break
		}
		params.Set("start_cursor", obj.NextCursor)
	}

	return blocks, nil
}

// GetBlock can get a block.
// ref: https://developers.notion.com/reference/retrieve-a-block
func (c *Client) GetBlock(ctx context.Context, blockID string) (*Block, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/blocks/%s", blockID), nil, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrBlockNotFound
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	}

	obj := &Block{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response: %v", err)
	}

	return obj, nil
}

// UpdateBlock can update a block
// ref: https://developers.notion.com/reference/update-a-block
func (c *Client) UpdateBlock(ctx context.Context, block *Block) (*Block, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(block); err != nil {
		return nil, fmt.Errorf("notion: failed to encode request body: %v", err)
	}
	req, err := c.newRequest(ctx, http.MethodPatch, fmt.Sprintf("/blocks/%s", block.ID), nil, buf)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrBlockNotFound
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &Block{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed to parse a reponse: %v", err)
	}

	return obj, nil
}

func (c *Client) DeleteBlock(ctx context.Context, blockID string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/blocks/%s", blockID), nil, nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusNotFound:
		return ErrBlockNotFound
	case http.StatusTooManyRequests:
		return ErrLimitExceeded
	}

	return nil
}

// CreatePage can create a page.
// ref: https://developers.notion.com/reference/post-page
func (c *Client) CreatePage(ctx context.Context, page *Page) (*Page, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(page); err != nil {
		return nil, fmt.Errorf("notion: failed to encode request body: %v", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/pages", nil, buf)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &Page{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response: %v", err)
	}

	return obj, nil
}

// UpdateProperties can add and update the property.
// ref: https://developers.notion.com/reference/patch-page
func (c *Client) UpdateProperties(ctx context.Context, pageID string, properties map[string]*PropertyData) (*Page, error) {
	body := struct {
		Properties map[string]*PropertyData `json:"properties"`
	}{
		Properties: properties,
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return nil, fmt.Errorf("notion: failed to encode request body: %v", err)
	}
	req, err := c.newRequest(ctx, http.MethodPatch, fmt.Sprintf("/pages/%s", pageID), nil, buf)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &Page{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response: %v", err)
	}

	return obj, nil
}

// AppendBlock is appending new children block.
// ref: https://developers.notion.com/reference/patch-block-children
func (c *Client) AppendBlock(ctx context.Context, blockID string, children []*Block) ([]*Block, error) {
	body := struct {
		Children []*Block `json:"children"`
	}{
		Children: children,
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return nil, fmt.Errorf("notion: failed to encode request body: %v", err)
	}
	req, err := c.newRequest(ctx, http.MethodPatch, fmt.Sprintf("/blocks/%s/children", blockID), nil, buf)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusTooManyRequests:
		return nil, ErrLimitExceeded
	}

	obj := &BlockList{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("failed parse a response: %v", err)
	}

	return obj.Results, nil
}

func (c *Client) Search(ctx context.Context, query string, sort *Sort) ([]Object, error) {
	body := struct {
		Query       string `json:"query"`
		Sort        *Sort  `json:"sort,omitempty"`
		StartCursor string `json:"start_cursor,omitempty"`
		PageSize    int    `json:"page_size"`
	}{
		Query:    query,
		Sort:     sort,
		PageSize: 100,
	}

	objs := make([]Object, 0)
	tmp := make([]Object, 0, 100)
	buf := new(bytes.Buffer)
	for {
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, err
		}

		req, err := c.newRequest(ctx, http.MethodPost, "/search", nil, buf)
		if err != nil {
			return nil, err
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest:
			res.Body.Close()
			return nil, ErrBadRequest
		case http.StatusTooManyRequests:
			res.Body.Close()
			return nil, ErrLimitExceeded
		}

		obj := &SearchResult{}
		if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
			res.Body.Close()
			return nil, fmt.Errorf("failed parse a response: %v", err)
		}
		res.Body.Close()

		meta := &Meta{}
		for _, v := range obj.Results {
			if err := json.Unmarshal(*v, meta); err != nil {
				return nil, err
			}

			switch meta.Object {
			case "database":
				db := &Database{}
				if err := json.Unmarshal(*v, db); err != nil {
					return nil, err
				}
				tmp = append(tmp, db)
			case "page":
				page := &Page{}
				if err := json.Unmarshal(*v, page); err != nil {
					return nil, err
				}
				tmp = append(tmp, page)
			default:
				return nil, fmt.Errorf("notion: unknown object type: %s", meta.Object)
			}
		}
		objs = append(objs, tmp...)
		tmp = tmp[:0]

		if !obj.HasMore {
			break
		}
		body.StartCursor = obj.NextCursor
	}

	return objs, nil
}

// CreateDatabase creates a database
// ref: https://developers.notion.com/reference/create-a-database
func (c *Client) CreateDatabase(ctx context.Context, db *Database) (*Database, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(db); err != nil {
		return nil, fmt.Errorf("notion: failed to encode request body: %v", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/databases", nil, buf)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	case http.StatusNotFound:
		return nil, ErrPageNotFound
	}

	obj := &Database{}
	if err := json.NewDecoder(res.Body).Decode(obj); err != nil {
		return nil, fmt.Errorf("notion: failed parse a response: %v", err)
	}

	return obj, nil
}

func (c *Client) newRequest(ctx context.Context, method string, apiPath string, params *url.Values, body io.Reader) (*http.Request, error) {
	u := &url.URL{}
	*u = *c.baseURL
	u.Path = path.Join(u.Path, apiPath)
	if params != nil {
		u.RawQuery = params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Notion-Version", notionVersion)
	req.Header.Add("User-Agent", UserAgent)
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	return req, nil
}
