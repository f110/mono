package main

import (
	"fmt"
	"os"
)

func controllerManager(args []string) error {
	c := New(args)

	return c.Loop()
}

func main() {
	if err := controllerManager(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
