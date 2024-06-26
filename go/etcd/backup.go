package etcd

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
)

type Backup struct {
	data       io.ReadCloser
	time       time.Time
	compressed *bytes.Buffer
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

func (b *Backup) Compressed() (io.Reader, error) {
	if b.data == nil && b.compressed != nil {
		return b.compressed, nil
	} else if b.data == nil {
		return nil, xerrors.New("internal: failed compress data?. Probably bug.")
	}

	writeBuffer := new(bytes.Buffer)
	w := zlib.NewWriter(writeBuffer)
	buf, err := ioutil.ReadAll(b.data)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	b.data.Close()
	b.data = nil

	if n, err := w.Write(buf); err != nil {
		return nil, xerrors.WithStack(err)
	} else if n != len(buf) {
		return nil, io.ErrShortWrite
	}

	b.compressed = writeBuffer
	return writeBuffer, nil
}

func (b *Backup) Time() time.Time {
	return b.time
}
