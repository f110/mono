package buildctl

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func triggerTask(endpoint string, jobId int, via string) error {
	v := &url.Values{}
	v.Add("job_id", strconv.Itoa(jobId))
	v.Add("via", via)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/run", endpoint), strings.NewReader(v.Encode()))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return xerrors.Errorf("failed trigger job: %s", res.Status)
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
	trigger := &cobra.Command{
		Use: "trigger",
		RunE: func(_ *cobra.Command, _ []string) error {
			return triggerTask(endpoint, jobId, via)
		},
	}
	trigger.Flags().StringVar(&endpoint, "endpoint", "", "API Endpoint")
	trigger.Flags().IntVar(&jobId, "job-id", 0, "Trigger job id")
	trigger.Flags().StringVar(&via, "via", "", "Via")
	job.AddCommand(trigger)

	rootCmd.AddCommand(job)
}
