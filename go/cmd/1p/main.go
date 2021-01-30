package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/pkg/cmd/onepassword"
	"go.f110.dev/mono/go/pkg/logger"
)

func onep() error {
	daemon := false
	rootCmd := &cobra.Command{
		Use:   "1p",
		Short: "The CLI for 1Password",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			logger.Init()
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if !daemon {
				cmd := exec.Command(os.Args[0], "--daemon")
				log.Print("start")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Start()
				time.Sleep(200 * time.Millisecond)
				return nil
			}

			log.Print("OK")
			log.Print(os.Getppid())
			log.Print(os.Getpid())
			log.Print(syscall.Getpgrp())
			syscall.Setsid()
			// syscall.Setpgid(0, 0)
			log.Print(syscall.Getpgrp())

			os.Stdout.Close()
			os.Stdin.Close()
			os.Stderr.Close()

			time.Sleep(10 * time.Second)

			return nil
		},
	}
	rootCmd.Flags().BoolVar(&daemon, "daemon", daemon, "Daemonize")

	onepassword.AddCommand(rootCmd)

	return rootCmd.Execute()
}

func main() {
	if err := onep(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
