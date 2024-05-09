package buildctl

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
)

func redoTask(endpoint string, taskId int, via string) error {
	v := &url.Values{}
	v.Add("job_id", strconv.Itoa(taskId))
	v.Add("via", via)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/run", endpoint), strings.NewReader(v.Encode()))
	if err != nil {
		return xerrors.WithStack(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return xerrors.Definef("failed trigger job: %s", res.Status).WithStack()
	}
	return nil
}

func Job(rootCmd *cli.Command) {
	job := &cli.Command{
		Use: "job",
	}

	endpoint := ""
	jobId := 0
	via := ""
	redo := &cli.Command{
		Use: "redo",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return redoTask(endpoint, jobId, via)
		},
	}
	redo.Flags().String("endpoint", "API Endpoint").Var(&endpoint)
	redo.Flags().Int("job-id", "Trigger task id").Var(&jobId)
	redo.Flags().String("via", "Via").Var(&via)
	job.AddCommand(redo)

	rootCmd.AddCommand(job)
}
