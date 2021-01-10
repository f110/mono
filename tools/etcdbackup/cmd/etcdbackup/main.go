package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/tools/etcdbackup/internal"
)

func etcdBackup(args []string) error {
	var endpoints []string
	var caCertPath string
	var certPath string
	var keyPath string
	var bucket string
	var pathPrefix string
	var credentialFile string

	fs := pflag.NewFlagSet("etcdbackup", pflag.ContinueOnError)
	fs.StringArrayVar(&endpoints, "endpoints", []string{}, "Endpoints of etcd")
	fs.StringVar(&caCertPath, "ca-cert", "", "CA Certificate file path")
	fs.StringVar(&certPath, "cert", "", "Client certificate file path")
	fs.StringVar(&keyPath, "key", "", "Private key file path")
	fs.StringVar(&bucket, "bucket", "", "Bucket name")
	fs.StringVar(&pathPrefix, "path-prefix", "", "Prefix")
	fs.StringVar(&credentialFile, "credential", "", "Credential file")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := logger.Init(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	var caCert *x509.Certificate
	if caCertPath != "" {
		c, err := internal.ReadCACertificate(caCertPath)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		caCert = c
	}
	var clientCert tls.Certificate
	if certPath != "" && keyPath != "" {
		c, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		clientCert = c
	}

	credential, err := ioutil.ReadFile(credentialFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	bu, err := internal.NewBackup(context.Background(), endpoints, caCert, clientCert)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	compressed, err := bu.Compressed()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	up := internal.NewUploader(credential, bucket)
	path := filepath.Join(pathPrefix, bu.Time().In(loc).Format("2006-01-02_15.zlib"))
	if err := up.Upload(context.Background(), compressed, path); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func main() {
	if err := etcdBackup(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
}
