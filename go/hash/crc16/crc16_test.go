package crc16

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCRC16ModBus(t *testing.T) {
	cases := []struct {
		Input    []byte
		Checksum uint16
	}{
		{
			Input:    []byte{0x01, 0x02},
			Checksum: 0xE181,
		},
	}

	for _, tt := range cases {
		got := ChecksumModBus(tt.Input)
		assert.Equal(t, tt.Checksum, got)
	}
}
