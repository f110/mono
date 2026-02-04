package buildctl

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/cli"
)

func Test(rootCmd *cli.Command) {
	var dir string
	test := &cli.Command{
		Use: "test",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			fileProvider := config.NewLocalProvider(dir)
			jobs, err := config.ReadJobsFromBuildDir(fileProvider)
			if err != nil {
				return err
			}
			for i, job := range jobs {
				if i != 0 {
					fmt.Println("---")
				}
				buf, err := yaml.Marshal(job)
				if err != nil {
					return err
				}
				fmt.Print(string(buf))
			}
			return nil
		},
	}
	test.Flags().String("config", "Config directory").Shorthand("c").Default(".build").Var(&dir)
	rootCmd.AddCommand(test)
}
