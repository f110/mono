package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/http/httplogger"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/vault"
)

func vaultClientSample() error {
	var vaultAddr, enginePath, role, serviceAccountTokenFile string
	cmd := &cli.Command{
		Use: "vault-client-sample",
		Run: func(ctx context.Context, cmd *cli.Command, args []string) error {
			debugTR := httplogger.New(http.DefaultTransport, true)
			client, err := vault.NewClientAsK8SServiceAccount(ctx, vaultAddr, enginePath, role, serviceAccountTokenFile, vault.HttpClient(&http.Client{Transport: debugTR}))
			if err != nil {
				return err
			}
			_ = client
			for {
				select {
				case <-time.After(1 * time.Minute):
					logger.Log.Info("Waiting...")
				case <-ctx.Done():
					return nil
				}
			}
			return nil
		},
	}
	cmd.Flags().String("addr", "").Var(&vaultAddr)
	cmd.Flags().String("engine-path", "").Var(&enginePath)
	cmd.Flags().String("role", "").Var(&role)
	cmd.Flags().String("service-account-token-file-path", "").Var(&serviceAccountTokenFile)

	return cmd.Execute(os.Args)
}

func main() {
	if err := vaultClientSample(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
