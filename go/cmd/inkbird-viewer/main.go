package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.f110.dev/mono/go/pkg/ble/inkbird"
)

func inkbirdViewer(args []string) error {
	if len(args) < 2 {
		return errors.New("Usage: inkbird-viewer id")
	}
	id := args[1]

	data, err := inkbird.Read(context.Background(), id)
	if err != nil {
		return err
	}

	fmt.Printf("Date: %s\n", data.Time.Format(time.RFC3339))
	fmt.Printf("Temp: %.2f\n", data.Temperature)
	fmt.Printf("Humid: %.2f\n", data.Humidity)
	fmt.Printf("Battery: %d%%\n", data.Battery)
	return nil
}

func main() {
	if err := inkbirdViewer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
