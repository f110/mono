package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	etcd2 "go.f110.dev/mono/go/etcd"
	"go.f110.dev/mono/go/storage"
)

func etcdBackup(endpoints []string, caCertPath, certPath, keyPath, bucket, pathPrefix, credentialFile string) error {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return xerrors.WithStack(err)
	}

	var caCert *x509.Certificate
	if caCertPath != "" {
		c, err := etcd2.ReadCACertificate(caCertPath)
		if err != nil {
			return xerrors.WithStack(err)
		}
		caCert = c
	}
	var clientCert tls.Certificate
	if certPath != "" && keyPath != "" {
		c, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return xerrors.WithStack(err)
		}
		clientCert = c
	}

	credential, err := os.ReadFile(credentialFile)
	if err != nil {
		return xerrors.WithStack(err)
	}

	bu, err := etcd2.NewBackup(context.Background(), endpoints, caCert, clientCert)
	if err != nil {
		return xerrors.WithStack(err)
	}
	up := storage.NewGCS(credential, bucket, storage.GCSOptions{})
	path := filepath.Join(pathPrefix, bu.Time().In(loc).Format("2006-01-02_15.zlib"))
	if err := up.PutReader(context.Background(), path, bu.Compressed()); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func main() {
	var endpoints []string
	var caCertPath, certPath, keyPath, bucket, pathPrefix, credentialFile string
	cmd := &cli.Command{
		Use: "etcdbackup",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			return etcdBackup(endpoints, caCertPath, certPath, keyPath, bucket, pathPrefix, credentialFile)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("endpoints", "Endpoints of etcd").Var(&endpoints)
	fs.String("ca-cert", "CA Certificate file path").Var(&caCertPath)
	fs.String("cert", "Client certificate file path").Var(&certPath)
	fs.String("key", "Private key file path").Var(&keyPath)
	fs.String("bucket", "Bucket name").Var(&bucket)
	fs.String("path-prefix", "Prefix").Var(&pathPrefix)
	fs.String("credential", "Credential file").Var(&credentialFile)
	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
}
