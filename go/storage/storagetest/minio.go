package storagetest

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/minio/minio-go/v7"
)

type MockMinIO struct {
	buckets []string
	objects map[string][]*minio.ObjectInfo

	removed map[string][]string
}

func NewMockMinIO() *MockMinIO {
	return &MockMinIO{
		objects: make(map[string][]*minio.ObjectInfo),
		removed: make(map[string][]string),
	}
}

func (m *MockMinIO) AddBucket(name string) {
	m.buckets = append(m.buckets, name)
	if _, ok := m.objects[name]; !ok {
		m.objects[name] = make([]*minio.ObjectInfo, 0)
	}
}

func (m *MockMinIO) AddObjects(bucket string, objs ...*minio.ObjectInfo) {
	m.objects[bucket] = append(m.objects[bucket], objs...)
}

func (m *MockMinIO) Removed(bucket string) []string {
	return m.removed[bucket]
}

func (m *MockMinIO) Transport(mock *httpmock.MockTransport) {
	buckets := strings.Join(m.buckets, "|")

	// Bucket location
	mock.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`.*/.+/\?location=$`),
		func(req *http.Request) (*http.Response, error) {
			s := strings.Split(req.URL.Path, "/")
			bucket := s[1]
			if _, ok := m.objects[bucket]; ok {
				return httpmock.NewStringResponse(http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?>
<LocationConstraint>
  <LocationConstraint>us-east-1</LocationConstraint>
</LocationConstraint>`), nil
			} else {
				return httpmock.NewXmlResponse(http.StatusNotFound, &minio.ErrorResponse{Code: "NoSuchBucket"})
			}
		},
	)

	// BucketExists
	mock.RegisterRegexpResponder(
		http.MethodHead,
		regexp.MustCompile(`.*/.+/$`),
		func(req *http.Request) (*http.Response, error) {
			s := strings.Split(req.URL.Path, "/")
			bucket := s[1]
			if _, ok := m.objects[bucket]; ok {
				return httpmock.NewXmlResponse(http.StatusOK, &minio.ErrorResponse{})
			} else {
				return httpmock.NewXmlResponse(http.StatusNotFound, &minio.ErrorResponse{Code: "NoSuchBucket"})
			}
		},
	)

	// Put Object
	mock.RegisterRegexpResponder(
		http.MethodPut,
		regexp.MustCompile(fmt.Sprintf(`.*/(%s)/`, buckets)),
		func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			if q.Get("partNumber") != "" && q.Get("uploadId") != "" {
				// Multipart upload
				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			}

			s := strings.Split(req.URL.Path, "/")
			bucket, key := s[1], strings.Join(s[2:], "/")
			found := false
			for _, v := range m.objects[bucket] {
				if v.Key == key {
					found = true
					break
				}
			}
			if !found {
				m.objects[bucket] = append(m.objects[bucket], &minio.ObjectInfo{
					Key: key,
				})
			}
			return httpmock.NewStringResponse(http.StatusOK, ""), nil
		},
	)

	// Put Object with multipart
	mock.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(fmt.Sprintf(`.*/(%s)/.+`, buckets)),
		func(req *http.Request) (*http.Response, error) {
			s := strings.Split(req.URL.Path, "/")
			bucket, key := s[1], strings.Join(s[2:], "/")
			body := initiateMultipartUploadResult{
				Bucket:   bucket,
				Key:      key,
				UploadID: "foobar",
			}
			return httpmock.NewXmlResponse(http.StatusOK, body)
		},
	)

	// ListObjects
	mock.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`.*/(%s)/\?.+list-type=2.+`, buckets)),
		func(req *http.Request) (*http.Response, error) {
			s := strings.Split(req.URL.Path, "/")
			bucket := s[1]

			objs := m.objects[bucket]
			contents := make([]ObjectInfo, len(objs))
			for i, v := range objs {
				contents[i] = ObjectInfo{
					ETag:         v.ETag,
					Key:          v.Key,
					LastModified: v.LastModified,
					Size:         v.Size,
					ContentType:  v.ContentType,
					Expires:      v.Expires,
					Metadata:     v.Metadata,
					Owner: Owner{
						DisplayName: v.Owner.DisplayName,
						ID:          v.Owner.ID,
					},
					Grant:             v.Grant,
					StorageClass:      v.StorageClass,
					IsLatest:          v.IsLatest,
					IsDeleteMarker:    v.IsDeleteMarker,
					VersionID:         v.VersionID,
					ReplicationStatus: v.ReplicationStatus,
					Expiration:        v.Expiration,
					ExpirationRuleID:  v.ExpirationRuleID,
					Err:               v.Err,
				}
			}
			return httpmock.NewXmlResponse(http.StatusOK, &ListBucketV2Result{Contents: contents})
		},
	)

	// RemoveObject
	mock.RegisterRegexpResponder(
		http.MethodDelete,
		regexp.MustCompile(`.*`),
		func(req *http.Request) (*http.Response, error) {
			s := strings.Split(req.URL.Path, "/")
			m.removed[s[1]] = append(m.removed[s[1]], strings.Join(s[2:], "/"))
			return httpmock.NewBytesResponse(http.StatusNoContent, nil), nil
		},
	)
}

type ListBucketV2Result struct {
	CommonPrefixes        []CommonPrefix
	Contents              []ObjectInfo
	Delimiter             string
	EncodingType          string
	IsTruncated           bool
	MaxKeys               int64
	Name                  string
	NextContinuationToken string
	ContinuationToken     string
	Prefix                string
	FetchOwner            string
	StartAfter            string
}

type CommonPrefix struct {
	Prefix string
}

type ObjectInfo struct {
	ETag              string      `json:"etag"`
	Key               string      `json:"name"`
	LastModified      time.Time   `json:"lastModified"`
	Size              int64       `json:"size"`
	ContentType       string      `json:"contentType"`
	Expires           time.Time   `json:"expires"`
	Metadata          http.Header `json:"metadata" xml:"-"`
	UserMetadata      StringMap   `json:"userMetadata"`
	UserTags          StringMap   `json:"userTags"`
	UserTagCount      int
	Owner             Owner
	Grant             []minio.Grant `xml:"Grant"`
	StorageClass      string        `json:"storageClass"`
	IsLatest          bool
	IsDeleteMarker    bool
	VersionID         string `xml:"VersionId"`
	ReplicationStatus string `xml:"ReplicationStatus"`
	Expiration        time.Time
	ExpirationRuleID  string
	Err               error `json:"-"`
}

type Owner struct {
	DisplayName string `json:"name"`
	ID          string `json:"id"`
}

type StringMap map[string]string

func (m *StringMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*m = StringMap{}
	type xmlMapEntry struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	}
	for {
		var e xmlMapEntry
		err := d.Decode(&e)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		(*m)[e.XMLName.Local] = e.Value
	}
	return nil
}

func (m *StringMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type xmlMapEntry struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	}
	var ent xmlMapEntry
	for k, v := range *m {
		ent.XMLName.Local = k
		ent.Value = v
		if err := e.Encode(ent); err != nil {
			return err
		}
	}

	return nil
}

type initiateMultipartUploadResult struct {
	Bucket   string
	Key      string
	UploadID string `xml:"UploadId"`
}
