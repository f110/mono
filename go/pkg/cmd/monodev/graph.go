package monodev

import (
	"bytes"
	"os"

	"github.com/spf13/cobra"
	"go.f110.dev/mono/go/pkg/fsm"
	"golang.org/x/xerrors"
)

func graph(dir string) error {
	buf := new(bytes.Buffer)
	if err := fsm.DotOutput(buf, dir); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf.WriteTo(os.Stdout)
	return nil
}

func Graph(rootCmd *cobra.Command) {
	graphCmd := &cobra.Command{
		Use:   "graph",
		Short: "Visualize FSM",
		RunE: func(_ *cobra.Command, args []string) error {
			return graph(args[0])
		},
	}

	rootCmd.AddCommand(graphCmd)
}
