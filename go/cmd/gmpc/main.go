package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
)

func authCmd(rootCmd *cli.Command) {
	cmd := &cli.Command{
		Use: "auth",
	}
	rootCmd.AddCommand(cmd)

	login := &cli.Command{
		Use: "login",
		Run: func(ctx context.Context, cmd *cli.Command, args []string) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:7589/start", nil)
			if err != nil {
				return xerrors.WithStack(err)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return xerrors.WithStack(err)
			}
			defer res.Body.Close()

			startReq := struct {
				VerificationURIComplete string `json:"verification_uri_complete"`
				DeviceCode              string `json:"device_code"`
			}{}
			if err := json.NewDecoder(res.Body).Decode(&startReq); err != nil {
				return xerrors.WithStack(err)
			}
			fmt.Printf("Open: %s\n", startReq.VerificationURIComplete)

			var accessToken string
			t := time.NewTicker(1 * time.Second)
		Wait:
			for {
				select {
				case <-t.C:
					req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:7589/start", nil)
					if err != nil {
						return xerrors.WithStack(err)
					}
					res, err := http.DefaultClient.Do(req)
					if err != nil {
						return xerrors.WithStack(err)
					}
					switch res.StatusCode {
					case http.StatusOK:
					default:
						res.Body.Close()
						continue
					}

					tokenRes := struct {
						AccessToken string `json:"access_token"`
					}{}
					if err := json.NewDecoder(res.Body).Decode(&tokenRes); err != nil {
						res.Body.Close()
						return xerrors.WithStack(err)
					}
					res.Body.Close()
					accessToken = tokenRes.AccessToken
					break Wait
				case <-ctx.Done():
					return nil
				}
			}
			fmt.Printf("Access token: %s\n", accessToken)
			confDir, err := os.UserConfigDir()
			if err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.MkdirAll(filepath.Join(confDir, "gmpc"), 0755); err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.WriteFile(filepath.Join(confDir, "gmpc", "access_token"), []byte(accessToken), 0644); err != nil {
				return xerrors.WithStack(err)
			}
			return nil
		},
	}
	cmd.AddCommand(login)
}

func GoModuleProxyCLI() error {
	cmd := &cli.Command{
		Use: "gmpc",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			if os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN") != "" {
				// Running on GitHub Actions
				return ghActions()
			}
			return localToken()
		},
	}
	authCmd(cmd)
	return cmd.Execute(os.Args)
}

func ghActions() error {
	return nil
}

func localToken() error {
	confDir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	accessToken, err := os.ReadFile(filepath.Join(confDir, "gmpc", "access_token"))
	if err != nil {
		return nil
	}
	fmt.Printf("http://localhost:7589/\n\nAuthorization: %s\n\n", string(accessToken))
	return nil
}

func main() {
	if err := GoModuleProxyCLI(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
