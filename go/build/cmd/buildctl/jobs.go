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

func Jobs(rootCmd *cli.Command, endpoint *string) {
	jobs := &cli.Command{
		Use: "jobs",
	}
	rootCmd.AddCommand(jobs)

	var repositoryID int
	list := &cli.Command{
		Use: "list",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}

			jobs, err := apiClient.ListJobs(ctx, api.RequestListJobs_builder{RepositoryId: varptr.Ptr(int32(repositoryID))}.Build())
			if err != nil {
				return err
			}
			table := tablewriter.NewWriter(os.Stdout)
			table.Header("Name")
			for _, v := range jobs.GetJobs() {
				table.Append([]string{v.GetName()})
			}
			return table.Render()
		},
	}
	list.Flags().Int("repository-id", "Repository ID").Var(&repositoryID).Required()
	jobs.AddCommand(list)

	var jobName string
	invoke := &cli.Command{
		Use: "invoke",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}

			job, err := apiClient.InvokeJob(ctx, api.RequestInvokeJob_builder{
				RepositoryId: varptr.Ptr(int32(repositoryID)),
				JobName:      &jobName,
			}.Build())
			if err != nil {
				return xerrors.WithStack(err)
			}
			fmt.Printf("OK: %d\n", job.GetTaskId())
			return nil
		},
	}
	invoke.Flags().Int("repository-id", "Repository ID").Var(&repositoryID).Required()
	invoke.Flags().String("name", "Job name").Var(&jobName)
	jobs.AddCommand(invoke)
}
