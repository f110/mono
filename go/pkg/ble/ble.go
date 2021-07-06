package ble

import "context"

type Peripheral struct {
	RSSI             int16
	Address          string
	Name             string
	ManufacturerData []byte
}

func Scan(ctx context.Context) <-chan Peripheral {
	return scan(ctx)
}
