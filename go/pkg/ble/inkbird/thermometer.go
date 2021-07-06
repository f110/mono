package inkbird

import (
	"context"
	"encoding/binary"
	"time"

	"go.f110.dev/mono/go/pkg/ble"
	"go.f110.dev/mono/go/pkg/hash/crc16"
	"golang.org/x/xerrors"
)

type ThermometerData struct {
	Time        time.Time
	Temperature float32
	Humidity    float32
	External    bool
	Battery     int8
}

func Read(ctx context.Context, id string) (*ThermometerData, error) {
	sCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	scanCh := ble.Scan(sCtx)
	var buf []byte
	for prph := range scanCh {
		if prph.Address == id && len(prph.ManufacturerData) == 9 {
			cancel()
			buf = prph.ManufacturerData
			break
		}
	}
	if buf == nil {
		return nil, xerrors.Errorf("inkbird: sensor not found")
	}

	temp := binary.LittleEndian.Uint16(buf[:2])
	humid := binary.LittleEndian.Uint16(buf[2:4])
	external := false
	if buf[5] == '1' {
		external = true
	}
	checksum := binary.LittleEndian.Uint16(buf[5:7])
	if checksum != crc16.ChecksumModBus(buf[:5]) {
		return nil, xerrors.Errorf("inkbird: Checksum mismatched")
	}
	battery := int8(buf[7])

	return &ThermometerData{
		Time:        time.Now(),
		Temperature: float32(temp) / 100,
		Humidity:    float32(humid) / 100,
		External:    external,
		Battery:     battery,
	}, nil
}
