package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	pf "k8s.io/client-go/tools/portforward"

	"go.f110.dev/mono/go/k8s/portforward"
	"go.f110.dev/mono/go/logger"
)

type S3Options struct {
	Region          string
	AccessKey       string
	SecretAccessKey string
	Endpoint        string
	PathStyle       bool // This is important option if you want to use with MinIO.
	CACertFile      string
	PartSize        uint64
	Retries         int

	Name      string
	Namespace string
	Port      int
	Dev       bool

	k8sClient    kubernetes.Interface
	restConfig   *rest.Config
	withInsecure bool

	client *s3.Client
}

func NewS3OptionToAWS(region, accessKey, secretAccessKey string) S3Options {
	return S3Options{
		Region:          region,
		AccessKey:       accessKey,
		SecretAccessKey: secretAccessKey,
	}
}

func NewS3OptionToExternal(endpoint, region, accessKey, secretAccessKey string) S3Options {
	return S3Options{
		Region:          region,
		AccessKey:       accessKey,
		SecretAccessKey: secretAccessKey,
		Endpoint:        endpoint,
	}
}

func NewS3OptionViaService(client kubernetes.Interface, config *rest.Config, name, namespace string, port int, accessKey, secretAccessKey string, dev bool) S3Options {
	return S3Options{
		Name:            name,
		Namespace:       namespace,
		Port:            port,
		AccessKey:       accessKey,
		SecretAccessKey: secretAccessKey,
		Dev:             dev,
		withInsecure:    true,
		k8sClient:       client,
		restConfig:      config,
	}
}

func (s *S3Options) Client(ctx context.Context) (*s3.Client, error) {
	if s.client != nil {
		return s.client, nil
	}

	if s.k8sClient != nil && s.Endpoint == "" {
		endpoint, forwarder, err := s.getS3Endpoint(ctx, s.Name, s.Namespace, s.Port)
		if err != nil {
			if forwarder != nil {
				forwarder.Close()
			}
			return nil, err
		}
		if !s.withInsecure {
			s.Endpoint = fmt.Sprintf("https://%s", endpoint)
		} else {
			s.Endpoint = fmt.Sprintf("http://%s", endpoint)
		}
	}

	cp, err := x509.SystemCertPool()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if s.CACertFile != "" {
		b, err := os.ReadFile(s.CACertFile)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		cp.AppendCertsFromPEM(b)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{RootCAs: cp}
	client := &http.Client{Transport: tr}

	credsProvider := credentials.NewStaticCredentialsProvider(s.AccessKey, s.SecretAccessKey, "")
	cfg := aws.Config{
		Region:      s.Region,
		HTTPClient:  client,
		Credentials: credsProvider,
		EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           s.Endpoint,
				SigningRegion: region,
			}, nil
		}),
	}
	var opts []func(options *s3.Options)
	if s.PathStyle {
		opts = append(opts, func(opt *s3.Options) {
			opt.UsePathStyle = true
		})
	}
	s.client = s3.NewFromConfig(cfg, opts...)

	return s.client, nil
}

func (s *S3Options) getS3Endpoint(ctx context.Context, name, namespace string, port int) (string, *pf.PortForwarder, error) {
	var forwarder *pf.PortForwarder
	var endpoint string
	var endpointService *corev1.Service
	if svc, err := s.k8sClient.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		return "", nil, xerrors.WithStack(err)
	} else {
		endpointService = svc
		endpoint = fmt.Sprintf("%s.%s.svc:%d", name, namespace, port)
	}
	if s.Dev {
		f, port, err := portforward.PortForward(ctx, endpointService, int(endpointService.Spec.Ports[0].Port), s.restConfig, s.k8sClient, nil)
		if err != nil {
			return "", nil, err
		}
		forwarder = f
		endpoint = fmt.Sprintf("127.0.0.1:%d", port)
	}

	return endpoint, forwarder, nil
}

func (s *S3Options) Uploader(ctx context.Context) (*manager.Uploader, error) {
	c, err := s.Client(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	u := manager.NewUploader(c)
	if s.PartSize > 0 {
		u.PartSize = int64(s.PartSize)
	}
	return u, nil
}

type S3 struct {
	bucket string
	opt    S3Options
}

var _ storageInterface = &S3{}

func NewS3(bucket string, opt S3Options) *S3 {
	return &S3{bucket: bucket, opt: opt}
}

func (s *S3) Name() string {
	return "s3"
}

func (s *S3) Endpoint() string {
	return s.opt.Endpoint
}

func (s *S3) Get(ctx context.Context, name string) (*Object, error) {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	retryCount := 1
	for {
		obj, err := c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(name),
		})
		if err != nil {
			if s.opt.Retries > 0 && retryCount < s.opt.Retries {
				logger.Log.Info("Retrying get a object", zap.Int("retryCount", retryCount), zap.String("key", name))
				retryCount++
				continue
			}

			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				switch apiErr.(type) {
				case *types.NoSuchKey:
					return nil, ErrObjectNotFound
				}
			}
			return nil, xerrors.WithStack(err)
		}

		return &Object{
			Name:         name,
			Size:         obj.ContentLength,
			LastModified: *obj.LastModified,
			Body:         obj.Body,
		}, nil
	}
}

func (s *S3) List(ctx context.Context, prefix string) ([]*Object, error) {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	paginator := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	var objs []*Object
	for paginator.HasMorePages() {
		var page *s3.ListObjectsV2Output
		retryCount := 1
		for {
			p, err := paginator.NextPage(ctx)
			if err != nil {
				if s.opt.Retries > 0 && retryCount < s.opt.Retries {
					logger.Log.Info("Retrying get a next page", zap.Int("retryCount", retryCount), zap.String("prefix", prefix))
					retryCount++
					continue
				}
				return nil, xerrors.WithStack(err)
			}
			page = p
			break
		}

		for _, v := range page.Contents {
			objs = append(objs, &Object{
				Name:         aws.ToString(v.Key),
				Size:         v.Size,
				LastModified: aws.ToTime(v.LastModified),
			})
		}
	}

	return objs, nil
}

func (s *S3) ListIter(ctx context.Context, prefix string) (iter.Seq[*Object], error) {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	paginator := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	return func(yield func(*Object) bool) {
		for paginator.HasMorePages() {
			var page *s3.ListObjectsV2Output
			retryCount := 1
			for {
				p, err := paginator.NextPage(ctx)
				if err != nil {
					if s.opt.Retries > 0 && retryCount < s.opt.Retries {
						logger.Log.Info("Retrying get a next page", zap.Int("retryCount", retryCount), zap.String("prefix", prefix))
						retryCount++
						continue
					}
					if !yield(&Object{Err: err}) {
						return
					}
				}
				page = p
				break
			}

			for _, v := range page.Contents {
				obj := &Object{
					Name:         aws.ToString(v.Key),
					Size:         v.Size,
					LastModified: aws.ToTime(v.LastModified),
				}
				if !yield(obj) {
					return
				}
			}
		}
	}, nil
}

func (s *S3) Put(ctx context.Context, name string, data []byte) error {
	return s.PutReader(ctx, name, bytes.NewReader(data))
}

func (s *S3) PutReader(ctx context.Context, name string, r io.Reader) error {
	uploader, err := s.opt.Uploader(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}
	retryCount := 1
	for {
		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(name),
			Body:   r,
		})
		if err != nil {
			if s.opt.Retries > 0 && retryCount < s.opt.Retries {
				logger.Log.Info("Retrying put a object", zap.Int("retryCount", retryCount), zap.String("key", name))
				retryCount++
				continue
			}
			return xerrors.WithStack(err)
		}

		return nil
	}
}

func (s *S3) Delete(ctx context.Context, name string) error {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}

	retryCount := 1
	for {
		_, err = c.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(name),
		})
		if err != nil {
			if s.opt.Retries > 0 && retryCount < s.opt.Retries {
				logger.Log.Info("Retrying delete a object", zap.Int("retryCount", retryCount), zap.String("key", name))
				retryCount++
				continue
			}
			return xerrors.WithStack(err)
		}

		return nil
	}
}

func (s *S3) MakeBucket(ctx context.Context, name string) error {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)})
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (s *S3) ExistBucket(ctx context.Context, name string) bool {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return false
	}

	_, err = c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(name)})
	if err != nil {
		return false
	}
	return true
}

func (s *S3) ExistObject(ctx context.Context, key string) bool {
	c, err := s.opt.Client(ctx)
	if err != nil {
		return false
	}

	_, err = c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false
	}
	return true
}
