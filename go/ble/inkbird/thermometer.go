package inkbird

import (
	"context"
	"encoding/binary"
	"log"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/ble"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/hash/crc16"
	"go.f110.dev/mono/go/logger"
)

type ThermometerData struct {
	Time        time.Time
	Temperature float32
	Humidity    float32
	External    bool
	Battery     int8
	RSSI        int16
}

var DefaultThermometerDataProvider = &ThermometerDataProvider{lastData: make(map[string]*ThermometerData)}

type ThermometerDataProvider struct {
	lastData map[string]*ThermometerData
}

func (t *ThermometerDataProvider) Get(id string) *ThermometerData {
	return t.lastData[id]
}

func (t *ThermometerDataProvider) Start(ctx context.Context) error {
	if err := ble.DefaultScanner.Start(ctx); err != nil {
		return xerrors.WithStack(err)
	}

	go func() {
		ch := ble.DefaultScanner.Scan()
		for {
			select {
			case prph := <-ch:
				if prph.Name == "sps" && len(prph.ManufacturerData) == 9 {
					d, err := readData(prph, prph.ManufacturerData)
					if err == nil {
						t.lastData[prph.Address] = d
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func readData(prph ble.Peripheral, buf []byte) (*ThermometerData, error) {
	temp := binary.LittleEndian.Uint16(buf[:2])
	humid := binary.LittleEndian.Uint16(buf[2:4])
	external := false
	if buf[5] == '1' {
		external = true
	}
	checksum := binary.LittleEndian.Uint16(buf[5:7])
	if checksum != crc16.ChecksumModBus(buf[:5]) {
		return nil, xerrors.Define("inkbird: Checksum mismatched").WithStack()
	}
	battery := int8(buf[7])

	return &ThermometerData{
		Time:        time.Now(),
		Temperature: float32(temp) / 100,
		Humidity:    float32(humid) / 100,
		External:    external,
		Battery:     battery,
		RSSI:        prph.RSSI,
	}, nil
}

func (t *ThermometerDataProvider) Stop() error {
	return ble.DefaultScanner.Stop()
}

func Read(ctx context.Context, id string) (*ThermometerData, error) {
	sCtx, cancel := ctxutil.WithCancel(ctx)
	defer cancel()

	scanCh, err := ble.Scan(sCtx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var peripheral ble.Peripheral
	var buf []byte
	for prph := range scanCh {
		logger.Log.Debug("Found device", zap.String("id", prph.Address), zap.Int("data_length", len(prph.ManufacturerData)))
		if prph.Address == id && len(prph.ManufacturerData) == 9 {
			cancel()
			buf = prph.ManufacturerData
			peripheral = prph
			break
		}
	}
	if buf == nil {
		return nil, xerrors.Define("inkbird: sensor not found").WithStack()
	}

	data, err := readData(peripheral, buf)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return data, nil
}

func Scan(ctx context.Context) error {
	sCtx, cancel := ctxutil.WithCancel(ctx)
	defer cancel()

	scanCh, err := ble.Scan(sCtx)
	if err != nil {
		return xerrors.WithStack(err)
	}

	for prph := range scanCh {
		if prph.Name == "sps" && len(prph.ManufacturerData) == 9 {
			log.Printf("Found sensor: %s", prph.Address)
		}
	}

	return nil
}
