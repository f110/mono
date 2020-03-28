package harbor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	userAgent = "harbor-client/1.0"
)

type roundtripper struct {
	username string
	password string
}

func (rt *roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.SetBasicAuth(rt.username, rt.password)
	r.Header.Set("User-Agent", userAgent)

	return http.DefaultTransport.RoundTrip(r)
}

type Harbor struct {
	host     string
	username string
	password string

	client *http.Client
}

func New(host, username, password string) *Harbor {
	h := &Harbor{
		host:     host,
		username: username,
		password: password,
		client:   &http.Client{},
	}
	h.client.Transport = &roundtripper{username: username, password: password}

	return h
}

func (h *Harbor) ListProjects() ([]Project, error) {
	req, err := h.newRequest(http.MethodGet, "projects", nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		// Succeeded
	case http.StatusUnauthorized:
		return nil, errors.New("unauthorized")
	default:
		return nil, fmt.Errorf("unknown status code: %d", res.StatusCode)
	}

	projects := make([]Project, 0)
	if err := json.NewDecoder(res.Body).Decode(&projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (h *Harbor) ExistProject(name string) (bool, error) {
	req, err := h.newRequest(http.MethodHead, "projects?project_name="+name, nil)
	if err != nil {
		return false, err
	}
	res, err := h.client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("exists project: unknown status code: %d", res.StatusCode)
	}
}

func (h *Harbor) NewProject(p *NewProjectRequest) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(p); err != nil {
		return err
	}

	req, err := h.newRequest(http.MethodPost, "projects", bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusCreated:
	// Succeeded
	case http.StatusConflict:
		return fmt.Errorf("%s already exists", p.ProjectName)
	default:
		return fmt.Errorf("new project: unknown status code: %d", res.StatusCode)
	}

	return nil
}

func (h *Harbor) newRequest(method string, endpoint string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequest(method, fmt.Sprintf("%s/api/%s", h.host, endpoint), body)
	if err != nil {
		return nil, err
	}
	r.SetBasicAuth(h.username, h.password)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "appliaction/json")

	return r, nil
}

type Project struct {
	Id       int         `json:"project_id,omitempty"`
	OwnerId  int         `json:"owner_id,omitempty"`
	Name     string      `json:"name"`
	Metadata ProjectMeta `json:"metadata"`
}

type ProjectMeta struct {
	Public bool `json:"public"`
}

type NewProjectRequest struct {
	ProjectName  string             `json:"project_name"`
	CVEWhitelist CVEWhitelist       `json:"cve_whitelist,omitempty"`
	CountLimit   int                `json:"count_limit,omitempty"`
	StorageLimit int                `json:"storage_limit,omitempty"`
	Metadata     NewProjectMetadata `json:"metadata,omitempty"`
}

type NewProjectMetadata struct {
	Public               string `json:"public,omitempty"`
	EnableContentTrust   string `json:"enable_content_trust,omitempty"`
	AutoScan             string `json:"auto_scan,omitempty"`
	Severity             string `json:"severity,omitempty"`
	ReuseSysCVEWhitelist string `json:"reuse_sys_cve_whitelist,omitempty"`
	PreventVUL           string `json:"prevent_vul,omitempty"`
}

type CVEWhitelist struct {
	Items []CVEItem `json:"items,omitempty"`
}

type CVEItem struct {
	CVEId string `json:"cve_id"`
}
