package logger

import (
	"fmt"
	"io"
)

type NamedWriter struct {
	name      []byte
	w         io.Writer
	lineStart bool
}

var _ io.Writer = &NamedWriter{}

func NewNamedWriter(w io.Writer, name string) *NamedWriter {
	return &NamedWriter{name: []byte(fmt.Sprintf("[%s] ", name)), w: w, lineStart: true}
}

func (w *NamedWriter) Write(b []byte) (int, error) {
	if w.lineStart {
		if err := w.outputHeader(); err != nil {
			return 0, err
		}
		w.lineStart = false
	}

	totalN := 0
	startIndex := 0
	for i := 0; i < len(b)-1; i++ {
		if b[i] == '\n' {
			n, err := w.w.Write(b[startIndex : i+1])
			if err != nil {
				return totalN, err
			}
			startIndex = i + 1
			totalN += n
			if err := w.outputHeader(); err != nil {
				return totalN, err
			}
		}
	}
	if startIndex != len(b)-1 {
		n, err := w.w.Write(b[startIndex:])
		if err != nil {
			return totalN + n, err
		}
		totalN += n
	}
	if b[len(b)-1] == '\n' {
		w.lineStart = true
	}

	return totalN, nil
}

func (w *NamedWriter) outputHeader() error {
	if _, err := w.w.Write(w.name); err != nil {
		return err
	}
	return nil
}
