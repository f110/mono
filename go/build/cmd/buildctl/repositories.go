package buildctl

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/varptr"
)

func Repositories(rootCmd *cli.Command, endpoint *string) {
	repo := &cli.Command{
		Use: "repositories",
	}
	rootCmd.AddCommand(repo)

	list := &cli.Command{
		Use: "list",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}

			repos, err := apiClient.ListRepositories(ctx, api.RequestListRepositories_builder{}.Build())
			if err != nil {
				return xerrors.WithStack(err)
			}
			table := tablewriter.NewWriter(os.Stdout)
			table.Header("Id", "Name", "URL", "Status")
			for _, v := range repos.GetRepositories() {
				table.Append([]string{fmt.Sprintf("%d", v.GetId()), v.GetName(), v.GetUrl(), v.GetStatus().String()})
			}
			return table.Render()
		},
	}
	repo.AddCommand(list)

	var repositoryID int
	sync := &cli.Command{
		Use: "sync",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}

			_, err = apiClient.SyncRepository(ctx, api.RequestSyncRepository_builder{RepositoryId: varptr.Ptr(int32(repositoryID))}.Build())
			if err != nil {
				return xerrors.WithStack(err)
			}
			fmt.Println("OK")
			return nil
		},
	}
	sync.Flags().Int("repository-id", "Repository ID").Var(&repositoryID).Required()
	repo.AddCommand(sync)
}
