package grafana

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/xerrors"
)

const (
	userAgent = "mono-api-client"
)

type Client struct {
	host     string
	user     string
	password string
}

func NewClient(host, user, password string) *Client {
	return &Client{host: host, user: user, password: password}
}

func (c *Client) Users() ([]*User, error) {
	u, err := url.Parse(c.host)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/users"

	req, err := c.newRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	users := make([]*User, 0)
	if err := json.NewDecoder(res.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}

func (c *Client) AddUser(user *User) error {
	u, err := url.Parse(c.host)
	if err != nil {
		return err
	}
	u.Path = "/api/admin/users"

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(user); err != nil {
		return err
	}
	req, err := c.newRequest(http.MethodPost, u.String(), buf)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return xerrors.Errorf("failed create user: %s", res.Status)
	}
	res.Body.Close()

	return nil
}

func (c *Client) DeleteUser(id int) error {
	u, err := url.Parse(c.host)
	if err != nil {
		return err
	}
	u.Path = fmt.Sprintf("/api/admin/users/%d", id)

	req, err := c.newRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return xerrors.Errorf("failed delete user: %s", res.Status)
	}

	return nil
}

func (c *Client) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

type User struct {
	Id            int
	Name          string
	Login         string
	Email         string
	Password      string
	IsAdmin       bool
	IsDisabled    bool
	LastSeenAt    time.Time
	LastSeenAtAge string
	AuthLabels    []string
}
