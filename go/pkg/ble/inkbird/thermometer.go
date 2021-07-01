package inkbird

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

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
}

func Read(ctx context.Context, id string) (*ThermometerData, error) {
	enableOnce.Do(func() {
		adapter.Enable()
	})

	addr, err := parseAddress(id)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	disconnected := false
	device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	defer func() {
		if !disconnected {
			device.Disconnect()
		}
	}()

	svcs, err := device.DiscoverServices([]bluetooth.UUID{ServiceUUIDThermometer})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if len(svcs) == 0 {
		return nil, xerrors.Errorf("inkbird: Thermometer service is not found")
	}

	svc := svcs[0]
	chars, err := svc.DiscoverCharacteristics([]bluetooth.UUID{CharacteristicUUIDRealtimeData})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if len(chars) == 0 {
		return nil, xerrors.Errorf("inkbird: Can not find real time data characteristic")
	}

	char := chars[0]
	buf := make([]byte, 1024)
	n, _ := char.Read(buf)
	if n != 7 {
		return nil, xerrors.Errorf("inkbird: expect 7 bytes but %d", n)
	}
	disconnected = true
	if err := device.Disconnect(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	temp := binary.LittleEndian.Uint16(buf[:2])
	humid := binary.LittleEndian.Uint16(buf[2:4])
	//unknown := binary.LittleEndian.Uint16(buf[5:7])

	return &ThermometerData{
		Time:        time.Now(),
		Temperature: float32(temp) / 100,
		Humidity:    float32(humid) / 100,
	}, nil
}
