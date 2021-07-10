package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"go.f110.dev/mono/go/pkg/ble/inkbird"
	"golang.org/x/xerrors"
)

func inkbirdViewer(args []string) error {
	if len(args) < 2 {
		return errors.New("Usage: inkbird-viewer id")
	}
	id := args[1]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := inkbird.DefaultThermometerDataProvider.Start(ctx)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	defer func() {
		cancel()
		err := inkbird.DefaultThermometerDataProvider.Stop()
		if err != nil {
			log.Print(err)
		}
	}()

	log.Print("sleep")
	time.Sleep(10 * time.Second)
	data := inkbird.DefaultThermometerDataProvider.Get(id)
	if data != nil {
		fmt.Printf("Date: %s\n", data.Time.Format(time.RFC3339))
		fmt.Printf("Temp: %.2f\n", data.Temperature)
		fmt.Printf("Humid: %.2f\n", data.Humidity)
		fmt.Printf("Battery: %d%%\n", data.Battery)
	}

	//err := ble.DefaultScanner.Start(ctx)
	//if err != nil {
	//	return xerrors.Errorf(": %w", err)
	//}
	//defer ble.DefaultScanner.Stop()
	//
	//ch := ble.DefaultScanner.Scan()
	//Scan:
	//	for {
	//		select {
	//		case prph := <-ch:
	//			fmt.Printf("Found device %s\n", prph.Address)
	//			if prph.Address != id || len(prph.ManufacturerData) != 9 {
	//				continue
	//			}
	//			fmt.Printf("%s\n", prph.Name)
	//		case <-ctx.Done():
	//			break Scan
	//		}
	//	}

	//data, err := inkbird.Read(context.Background(), id)
	//if err != nil {
	//	return err
	//}
	//
	//fmt.Printf("Date: %s\n", data.Time.Format(time.RFC3339))
	//fmt.Printf("Temp: %.2f\n", data.Temperature)
	//fmt.Printf("Humid: %.2f\n", data.Humidity)
	//fmt.Printf("Battery: %d%%\n", data.Battery)
	return nil
}

func main() {
	if err := inkbirdViewer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
