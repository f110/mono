package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/artifactupdater"
	"go.f110.dev/mono/go/cli"
	githubutil "go.f110.dev/mono/go/github"
)

func artifactUpdater(args []string) error {
	var authorName, authorEmail string
	var natsURL, streamName, subject, consumerName string
	var buildAPI string
	var appID, installationID int64
	var privateKeyFile string
	var registryInsecure bool
	cmd := &cli.Command{
		Use: "build-artifact-updater",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			var tokenSource artifactupdater.TokenSource
			if privateKeyFile != "" {
				app, err := githubutil.NewApp(appID, installationID, privateKeyFile)
				if err != nil {
					return err
				}
				tokenSource = app
			}
			conn, err := grpc.NewClient(buildAPI, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return xerrors.WithStack(err)
			}
			defer conn.Close()
			registry, err := artifactupdater.NewORASRegistry(registryInsecure)
			if err != nil {
				return err
			}

			u := artifactupdater.NewUpdater(authorName, authorEmail, tokenSource, api.NewAPIClient(conn), registry)
			return u.Run(ctx, natsURL, streamName, subject, consumerName)
		},
	}
	fs := cmd.Flags()
	fs.String("author-name", "Name of the author of the commits").Var(&authorName).Required()
	fs.String("author-email", "Email of the author of the commits").Var(&authorEmail).Required()
	fs.String("nats-url", "The URL for nats-server").Var(&natsURL).Required()
	fs.String("nats-stream-name", "The name of stream for JetStream").Var(&streamName).Default("build")
	fs.String("nats-subject", "The subject of stream").Var(&subject).Default("artifact.published")
	fs.String("consumer-name", "The name of the durable consumer").Var(&consumerName).Default("artifact-updater")
	fs.String("build-api", "The address of the API of Build").Var(&buildAPI).Required()
	fs.Int64("github-app-id", "GitHub App Id").Var(&appID)
	fs.Int64("github-installation-id", "GitHub Installation Id").Var(&installationID)
	fs.String("private-key-file", "GitHub app private key file. If empty, the repositories are accessed without authentication").Var(&privateKeyFile)
	fs.Bool("artifact-registry-insecure", "Allow accessing the artifact registries over plain HTTP").Var(&registryInsecure)

	return cmd.Execute(args)
}

func main() {
	if err := artifactUpdater(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
