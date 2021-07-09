package ble

import (
	"context"
	"strings"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"go.f110.dev/mono/go/pkg/logger"
	"go.uber.org/zap"
)

func scan(ctx context.Context) (<-chan Peripheral, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		d.Stop()
	}()
	ble.SetDefaultDevice(d)

	ch := make(chan Peripheral)
	go func() {
		defer close(ch)
		err = ble.Scan(ctx, false, func(a ble.Advertisement) {
			ch <- Peripheral{
				Address:          strings.ToLower(a.Addr().String()),
				Name:             a.LocalName(),
				RSSI:             int16(a.RSSI()),
				ManufacturerData: a.ManufacturerData(),
			}
		}, nil)
		if err != nil && err != context.Canceled {
			logger.Log.Warn("Failed to scan a device", zap.Error(err))
		}
	}()

	return ch, nil
}
