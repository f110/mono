package buildctl

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/cli"
)

func Tasks(rootCmd *cli.Command, endpoint *string) {
	tasks := &cli.Command{
		Use: "tasks",
	}
	rootCmd.AddCommand(tasks)

	var repositoryID int
	list := &cli.Command{
		Use: "list",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}

			builder := api.RequestListTasks_builder{}
			if repositoryID != 0 {
				builder.RepositoryIds = []int32{int32(repositoryID)}
			}
			tasks, err := apiClient.ListTasks(ctx, builder.Build())
			if err != nil {
				return xerrors.WithStack(err)
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.Header("ID", "Started At", "Finished At")
			for _, v := range tasks.GetTasks() {
				table.Append([]string{
					fmt.Sprintf("%d", v.GetId()),
					v.GetStartAt().AsTime().Format("2006-01-02 15:04:05"),
					v.GetFinishedAt().AsTime().Format("2006-01-02 15:04:05"),
				})
			}
			return table.Render()
		},
	}
	list.Flags().Int("repository-id", "Repository ID").Var(&repositoryID)
	tasks.AddCommand(list)

	var taskID int
	show := &cli.Command{
		Use: "show",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}

			builder := api.RequestListTasks_builder{}
			builder.Ids = []int32{int32(taskID)}
			tasks, err := apiClient.ListTasks(ctx, builder.Build())
			if err != nil {
				return xerrors.WithStack(err)
			}
			if len(tasks.GetTasks()) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}
			task := tasks.GetTasks()[0]

			repos, err := apiClient.ListRepositories(ctx, api.RequestListRepositories_builder{Ids: []int32{task.GetRepositoryId()}}.Build())
			if err != nil {
				return xerrors.WithStack(err)
			}
			repo := repos.GetRepositories()[0]

			fmt.Printf("ID: %d\n", task.GetId())
			fmt.Printf("Repository: %s\n", repo.GetName())
			fmt.Printf("Command: %s\n", task.GetCommand())
			fmt.Printf("Targets: %v\n", task.GetTargets())
			fmt.Printf("Started At: %s\n", task.GetStartAt().AsTime().Format("2006-01-02 15:04:05"))
			return nil
		},
	}
	show.Flags().Int("task-id", "Task ID").Var(&taskID).Required()
	tasks.AddCommand(show)
}
