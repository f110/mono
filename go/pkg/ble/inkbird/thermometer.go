package inkbird

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"go.f110.dev/mono/go/pkg/hash/crc16"
	"golang.org/x/xerrors"
	"tinygo.org/x/bluetooth"
)

var (
	adapter    = bluetooth.DefaultAdapter
	enableOnce = sync.Once{}

	ServiceUUIDThermometer         = bluetooth.New16BitUUID(0xFFF0)
	CharacteristicUUIDRealtimeData = bluetooth.New16BitUUID(0xFFF2)
)

type ThermometerData struct {
	Time        time.Time
	Temperature float32
	Humidity    float32
	External    bool
	Battery     int8
}

func Read(ctx context.Context, id string) (*ThermometerData, error) {
	enableOnce.Do(func() {
		adapter.Enable()
	})

	var buf []byte
	err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
		if result.Address.String() == id && len(result.ManufacturerData()) == 9 {
			buf = result.ManufacturerData()
			adapter.StopScan()
		}
		select {
		case <-ctx.Done():
			adapter.StopScan()
		default:
		}
	})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
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
	battery := 100 - int8(buf[8])

	return &ThermometerData{
		Time:        time.Now(),
		Temperature: float32(temp) / 100,
		Humidity:    float32(humid) / 100,
		External:    external,
		Battery:     battery,
	}, nil
}
