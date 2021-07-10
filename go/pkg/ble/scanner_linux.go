package ble

import (
	"context"
	"strings"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

func (s *Scanner) start(ctx context.Context) error {
	d, err := linux.NewDevice()
	if err != nil {
		return err
	}
	ble.SetDefaultDevice(d)

	go func() {
		err := ble.Scan(ctx, true, s.foundAdvertisement, nil)
		if err != nil && err != context.Canceled {
			s.Error = err
		}
	}()

	return nil
}

func (s *Scanner) stop() error {
	return ble.Stop()
}

func (s *Scanner) foundAdvertisement(a ble.Advertisement) {
	prph := Peripheral{
		Address:          strings.ToLower(a.Addr().String()),
		Name:             a.LocalName(),
		RSSI:             int16(a.RSSI()),
		ManufacturerData: a.ManufacturerData(),
	}

	s.mu.Lock()
	channels := s.ch
	s.mu.Unlock()
	for _, v := range channels {
		select {
		case v <- prph:
		default:
		}
	}
}
