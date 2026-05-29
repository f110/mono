package main

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestDecodeKey(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want keyAction
	}{
		{"arrow up", []byte{0x1b, '[', 'A'}, actionUp},
		{"arrow down", []byte{0x1b, '[', 'B'}, actionDown},
		{"k", []byte{'k'}, actionUp},
		{"j", []byte{'j'}, actionDown},
		{"ctrl-p", []byte{0x10}, actionUp},
		{"ctrl-n", []byte{0x0e}, actionDown},
		{"enter CR", []byte{'\r'}, actionConfirm},
		{"enter LF", []byte{'\n'}, actionConfirm},
		{"ctrl-c", []byte{0x03}, actionAbort},
		{"esc alone", []byte{0x1b}, actionAbort},
		{"q", []byte{'q'}, actionAbort},
		{"unknown single", []byte{'x'}, actionNone},
		{"empty", []byte{}, actionNone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeKey(tc.in)
			assertion.Equal(t, got, tc.want)
		})
	}
}
