package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/logger"
)

func goModuleProxy(args []string) error {
	proxy := newGoModuleProxyCommand()

	cmd := &cobra.Command{
		Use: "gomodule-proxy",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := logger.Init(); err != nil {
				return err
			}
			if err := proxy.Init(); err != nil {
				return err
			}
			return proxy.Run()
		},
	}
	logger.Flags(cmd.Flags())
	proxy.Flags(cmd.Flags())
	for _, v := range proxy.RequiredFlags() {
		if err := cmd.MarkFlagRequired(v); err != nil {
			return err
		}
	}

	cmd.SetArgs(args)
	return cmd.Execute()
}

func main() {
	if err := goModuleProxy(os.Args); err != nil {
		format := "%v\n"
		if os.Getenv("DEBUG") != "" {
			format = "%+v\n"
		}
		fmt.Fprintf(os.Stderr, format, err)
		os.Exit(1)
	}
}
