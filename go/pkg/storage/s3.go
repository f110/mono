package storage

import (
	"bytes"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/xerrors"
)

type S3Options struct {
	Region          string
	AccessKey       string
	SecretAccessKey string

	Endpoint string

	PathStyle bool

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

func (s *S3Options) Client() (*s3.Client, error) {
	if s.client != nil {
		return s.client, nil
	}

	credsProvider := credentials.NewStaticCredentialsProvider(s.AccessKey, s.SecretAccessKey, "")
	cfg := aws.Config{
		Region:      s.Region,
		Credentials: credsProvider,
		EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: s.Endpoint,
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

func (s *S3Options) Uploader() (*manager.Uploader, error) {
	c, err := s.Client()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return manager.NewUploader(c), nil
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

func (s *S3) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	c, err := s.opt.Client()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	obj, err := c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return obj.Body, nil
}

func (s *S3) List(ctx context.Context, prefix string) ([]*Object, error) {
	c, err := s.opt.Client()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	paginator := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	var objs []*Object
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
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

func (s *S3) Put(ctx context.Context, name string, data []byte) error {
	return s.PutReader(ctx, name, bytes.NewReader(data))
}

func (s *S3) PutReader(ctx context.Context, name string, r io.Reader) error {
	uploader, err := s.opt.Uploader()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
		Body:   r,
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (s *S3) Delete(ctx context.Context, name string) error {
	c, err := s.opt.Client()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	_, err = c.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}
