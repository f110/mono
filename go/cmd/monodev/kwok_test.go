package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"testing"

	"k8s.io/client-go/tools/clientcmd"

	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestKwokPKI(t *testing.T) {
	assertion.MustNoError(t, slogger.Init())
	t.Setenv("BUILD_WORKING_DIRECTORY", t.TempDir())

	kwokPKI.Run(context.Background())

	for _, name := range []string{"kwok.crt", "kwok.key", "sa.key", "sa.pub", "token.csv", "kubeconfig"} {
		assertion.True(t, fileExists(kwokFile(name)))
	}

	// The serving certificate must be usable both as kwok's kubelet cert and as
	// the CA the apiserver trusts, and must carry the node IP as a SAN.
	buf, err := os.ReadFile(kwokFile("kwok.crt"))
	assertion.MustNoError(t, err)
	block, _ := pem.Decode(buf)
	assertion.True(t, block != nil)
	cert, err := x509.ParseCertificate(block.Bytes)
	assertion.MustNoError(t, err)
	assertion.True(t, cert.IsCA)
	hasNodeIP := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == kwokNodeIP {
			hasNodeIP = true
		}
	}
	assertion.True(t, hasNodeIP)

	cfg, err := clientcmd.LoadFromFile(kwokFile("kubeconfig"))
	assertion.MustNoError(t, err)
	cluster := cfg.Clusters["kwok"]
	assertion.True(t, cluster != nil)
	assertion.Equal(t, cluster.Server, fmt.Sprintf("https://127.0.0.1:%d", kwokAPIServerPort))
	assertion.True(t, cluster.InsecureSkipTLSVerify)
	assertion.Equal(t, cfg.AuthInfos["dev"].Token, kwokAuthToken)
}
