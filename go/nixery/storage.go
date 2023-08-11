package nixery

import (
	"bytes"
	"context"
	"io"
	"net/http"

	nstorage "github.com/google/nixery/storage"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/storage"
)

type S3 struct {
	bucket  string
	backend *storage.S3
}

var _ nstorage.Backend = &S3{}

func NewS3Storage(endpoint, region, accessKey, secretAccessKey, bucket, caCertFile string) *S3 {
	opt := storage.NewS3OptionToExternal(endpoint, region, accessKey, secretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = caCertFile
	s := storage.NewS3(bucket, opt)
	return &S3{bucket: bucket, backend: s}
}

func (s *S3) Name() string {
	return "S3 (" + s.bucket + ")"
}

// Persist provides a user-supplied function with a writer
// that stores data in the storage backend.
//
// It needs to return the SHA256 hash of the data written as
// well as the total number of bytes, as those are required
// for the image manifest.
func (s *S3) Persist(ctx context.Context, path, _ string, f nstorage.Persister) (string, int64, error) {
	buf := new(bytes.Buffer)
	hash, size, err := f(buf)
	if err != nil {
		return "", 0, xerrors.WithStack(err)
	}
	if err := s.backend.Put(ctx, path, buf.Bytes()); err != nil {
		return "", 0, err
	}

	return hash, size, nil
}

// Fetch retrieves data from the storage backend.
func (s *S3) Fetch(ctx context.Context, path string) (io.ReadCloser, error) {
	obj, err := s.backend.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	return obj.Body, nil
}

// Move renames a path inside the storage backend. This is
// used for staging uploads while calculating their hashes.
func (s *S3) Move(ctx context.Context, old, new string) error {
	obj, err := s.backend.Get(ctx, old)
	if err != nil {
		return err
	}
	defer obj.Body.Close()

	if err := s.backend.PutReader(ctx, new, obj.Body); err != nil {
		return err
	}
	return nil
}

// Serve provides a handler function to serve HTTP requests
// for objects in the storage backend.
func (s *S3) Serve(digest string, req *http.Request, w http.ResponseWriter) error {
	obj, err := s.backend.Get(req.Context(), "")
	if err != nil {
		return err
	}
	defer obj.Body.Close()
	if _, err := io.Copy(w, obj.Body); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}
