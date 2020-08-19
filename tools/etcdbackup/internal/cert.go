package internal

import (
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/xerrors"
	"io/ioutil"
)

func ReadCACertificate(path string) (*x509.Certificate, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
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
		return nil, xerrors.Errorf("internal: %s not contain certificate", path)
	}

	cer, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return cer, nil
}
