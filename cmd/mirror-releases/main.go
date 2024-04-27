package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type command struct {
	Endpoint        string
	Region          string
	AccessKey       string
	SecretAccessKey string
	Bucket          string
	CAFile          string
	Prefix          string
	MetadataPrefix  string

	client     *storage.S3
	downloader *downloader
}

func newCommand() *command {
	return &command{}
}

func (c *command) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Endpoint, "endpoint", "", "")
	fs.StringVar(&c.Bucket, "bucket", "", "The bucket name")
	fs.StringVar(&c.Region, "region", "", "")
	fs.StringVar(&c.AccessKey, "access-key", "", "")
	fs.StringVar(&c.SecretAccessKey, "secret-access-key", "", "")
	fs.StringVar(&c.CAFile, "ca-file", "", "File path that contains CA certificate")
	fs.StringVar(&c.Prefix, "prefix", "", "")
	fs.StringVar(&c.MetadataPrefix, "metadata", "_mirror_metadata", "")
}

func (c *command) Run(ctx context.Context) error {
	opt := storage.NewS3OptionToExternal(c.Endpoint, c.Region, c.AccessKey, c.SecretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = c.CAFile
	c.client = storage.NewS3(c.Bucket, opt)
	d, err := newDownloader()
	if err != nil {
		return err
	}
	c.downloader = d

	for _, v := range mirrorTargets {
		if err := c.mirror(ctx, v); err != nil {
			return err
		}
	}
	return nil
}

func (c *command) mirror(ctx context.Context, target *mirrorAsset) error {
	if c.client.ExistObject(ctx, path.Join(c.MetadataPrefix, url.PathEscape(target.OriginURL))) {
		logger.Log.Debug("File exist. Skip mirroring", zap.String("url", target.OriginURL))
		return nil
	}

	logger.Log.Debug("File not found. we're going to get the file", zap.String("url", target.OriginURL))
	var checksums []*fileChecksum
	if target.ChecksumURL != "" {
		if v, err := c.getChecksums(ctx, target.ChecksumURL); err != nil {
			return err
		} else {
			checksums = v
		}
	}

	// Fetch origin file
	body, filename, err := c.downloader.Fetch(ctx, target.OriginURL)
	if err != nil {
		return err
	}

	// Verify the file checksum if the checksum url is specified
	if len(checksums) > 0 {
		var checksum *fileChecksum
		for _, v := range checksums {
			if v.Filename == filename {
				checksum = v
				break
			}
		}
		if checksum == nil {
			return xerrors.Definef("checksum for %s not found", filename).WithStack()
		}
		switch target.ChecksumAlgorithm {
		case SHA256Checksum:
			hasher := sha256.New()
			if _, err := io.Copy(hasher, body); err != nil {
				return xerrors.WithStack(err)
			}
			h := hasher.Sum(nil)
			fileHash := hex.EncodeToString(h)
			if fileHash != checksum.Hash {
				return xerrors.Definef("%s checksum mismatched. expected: %s calculated: %s", target.OriginURL, checksum.Hash, fileHash).WithStack()
			}
			body.Seek(0, io.SeekStart)
		}
	}

	// Verify the signature if the signature url is provided
	if target.SignatureURL != "" {
		sigFile, _, err := c.downloader.Fetch(ctx, target.SignatureURL)
		if err != nil {
			return err
		}
		defer sigFile.Close()
		rawPubKey, _, err := c.downloader.Fetch(ctx, target.PublicKeyURL)
		if err != nil {
			return err
		}
		entityList, err := openpgp.ReadArmoredKeyRing(rawPubKey)
		if err != nil {
			return xerrors.WithStack(err)
		}
		defer rawPubKey.Close()
		_, _, err = openpgp.VerifyDetachedSignature(entityList, body, sigFile, &packet.Config{})
		if err != nil {
			return xerrors.WithStack(err)
		}
		body.Seek(0, io.SeekStart)
	}

	logger.Log.Info("Put data", zap.String("path", path.Join(target.Path, filename)))
	err = c.client.PutReader(ctx, path.Join(c.Prefix, target.Path, filename), body)
	if err != nil {
		return err
	}

	logger.Log.Debug("Make metadata", zap.String("path", path.Join(c.MetadataPrefix, url.PathEscape(target.OriginURL))))
	return c.client.Put(ctx, path.Join(c.MetadataPrefix, url.PathEscape(target.OriginURL)), []byte(path.Join(c.Prefix, target.Path, filename)))
}

func (c *command) getChecksums(ctx context.Context, checksumURL string) ([]*fileChecksum, error) {
	checksumFile, _, err := c.downloader.Fetch(ctx, checksumURL)
	if err != nil {
		return nil, err
	}
	defer checksumFile.Close()

	return parseChecksumFile(checksumFile)
}

type fileChecksum struct {
	Hash     string
	Binary   bool
	Filename string
}

func parseChecksumFile(r io.Reader) ([]*fileChecksum, error) {
	var checksums []*fileChecksum
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		var hash, filename string
		var mode rune
		i := strings.Index(line, " ")
		if i == -1 {
			return nil, xerrors.New("invalid format")
		}
		hash = line[:i]
		mode = rune(line[i+1])
		filename = line[i+2:]
		checksums = append(checksums, &fileChecksum{
			Hash:     hash,
			Binary:   mode == '*',
			Filename: filename,
		})
	}

	return checksums, nil
}

type ChecksumAlgorithm int

const (
	SHA256Checksum ChecksumAlgorithm = iota
)

type mirrorAsset struct {
	Path              string
	OriginURL         string
	ChecksumURL       string
	ChecksumAlgorithm ChecksumAlgorithm
	SignatureURL      string
	PublicKeyURL      string
}

var mirrorTargets []*mirrorAsset

const (
	requestAgent = "in-house agent: github.com/f110"
)

type downloader struct {
	dir    string
	client *http.Client
}

func newDownloader() (*downloader, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &downloader{dir: dir, client: &http.Client{}}, nil
}

func (d *downloader) Fetch(ctx context.Context, rawURL string) (io.ReadSeekCloser, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	localPath := filepath.Join(d.dir, u.Host+"_"+strings.ReplaceAll(path.Clean(u.Path), "/", "_"))
	filenameFile := localPath + ".name"

	if _, err := os.Lstat(localPath); err == nil {
		f, err := os.Open(localPath)
		if err != nil {
			return nil, "", xerrors.WithStack(err)
		}
		filename, err := os.ReadFile(filenameFile)
		if err != nil {
			return nil, "", xerrors.WithStack(err)
		}
		return f, string(filename), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	req.Header.Add("User-Agent", requestAgent)
	logger.Log.Debug("HTTP Request", zap.String("url", req.URL.String()))
	res, err := d.client.Do(req)
	if err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		res.Body.Close()
		return nil, "", xerrors.Definef("request error %s", res.Status).WithStack()
	}

	var filename string
	if v := res.Header.Get("Content-Disposition"); v != "" {
		_, params, err := mime.ParseMediaType(v)
		if err != nil {
			return nil, "", xerrors.WithStack(err)
		}
		if n := params["filename"]; n != "" {
			filename = n
		}
	} else {
		filename = filepath.Base(u.Path)
	}
	temporaryFile := localPath + ".tmp"
	cacheFile, err := os.Create(temporaryFile)
	if err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	var fileSize int64
	if n, err := io.Copy(cacheFile, res.Body); err != nil {
		return nil, "", xerrors.WithStack(err)
	} else {
		fileSize = n
	}
	if err := cacheFile.Sync(); err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	if _, err := cacheFile.Seek(0, io.SeekStart); err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	logger.Log.Debug("Rename cache file", zap.String("new", localPath), zap.Int64("size", fileSize))
	if err := os.Rename(temporaryFile, localPath); err != nil {
		return nil, "", xerrors.WithStack(err)
	}
	if err := os.WriteFile(filenameFile, []byte(filename), 0644); err != nil {
		return nil, "", xerrors.WithStack(err)
	}

	return cacheFile, filename, nil
}

func (d *downloader) Close() {
	os.RemoveAll(d.dir)
}

func mirrorReleases() error {
	c := newCommand()
	cmd := &cobra.Command{
		Use: "mirror-releases",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return logger.Init()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.Run(cmd.Context())
		},
	}
	logger.Flags(cmd.Flags())
	c.Flags(cmd.Flags())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cmd.ExecuteContext(ctx)
}

func main() {
	if err := mirrorReleases(); err != nil {
		os.Exit(1)
	}
}
