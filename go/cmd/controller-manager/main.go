package main

import (
	"fmt"
	"os"

	"go.f110.dev/mono/go/pkg/cmd/controllers"
)

func controllerManager(args []string) error {
	c := controllers.New(args)

	return c.Loop()
}

func main() {
	if err := controllerManager(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
