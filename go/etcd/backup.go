package etcd

import (
	"compress/zlib"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
)

type Backup struct {
	data io.ReadCloser
	time time.Time
}

func NewBackup(ctx context.Context, endpoints []string, caCert *x509.Certificate, cert tls.Certificate) (*Backup, error) {
	systemPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if caCert != nil {
		systemPool.AddCert(caCert)
	}
	var certs []tls.Certificate
	if cert.Certificate != nil {
		certs = append(certs, cert)
	}

	cfg := clientv3.Config{
		Endpoints: endpoints,
		TLS: &tls.Config{
			Certificates: certs,
			RootCAs:      systemPool,
		},
	}
	etcdClient, err := clientv3.New(cfg)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	data, err := etcdClient.Snapshot(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	logger.Log.Info("Succeeded snapshot")
	return &Backup{data: data, time: time.Now()}, nil
}

// Compressed returns the reader of the snapshot compressed by zlib.
// The snapshot is compressed while the reader is read, so the whole snapshot is not held in memory.
// The returned reader can be read only once.
func (b *Backup) Compressed() io.Reader {
	pr, pw := io.Pipe()
	go func() {
		w := zlib.NewWriter(pw)
		_, err := io.Copy(w, b.data)
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		b.data.Close()
		if err != nil {
			err = xerrors.WithStack(err)
		}
		pw.CloseWithError(err)
	}()
	return pr
}

func (b *Backup) Time() time.Time {
	return b.time
}
