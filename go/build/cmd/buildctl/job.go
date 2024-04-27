package buildctl

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"
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

func Job(rootCmd *cobra.Command) {
	job := &cobra.Command{
		Use: "job",
	}

	endpoint := ""
	jobId := 0
	via := ""
	redo := &cobra.Command{
		Use: "redo",
		RunE: func(_ *cobra.Command, _ []string) error {
			return redoTask(endpoint, jobId, via)
		},
	}
	redo.Flags().StringVar(&endpoint, "endpoint", "", "API Endpoint")
	redo.Flags().IntVar(&jobId, "job-id", 0, "Trigger task id")
	redo.Flags().StringVar(&via, "via", "", "Via")
	job.AddCommand(redo)

	rootCmd.AddCommand(job)
}
