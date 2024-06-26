package etcd

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"

	"go.f110.dev/xerrors"
)

func ReadCACertificate(path string) (*x509.Certificate, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var data []byte
	for {
		block, rest := pem.Decode(b)
		b = rest
		if block.Type == "CERTIFICATE" {
			data = block.Bytes
			break
		}
		if len(rest) == 0 {
			break
		}
	}
	if data == nil {
		return nil, xerrors.Definef("internal: %s not contain certificate", path).WithStack()
	}

	cer, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return cer, nil
}
