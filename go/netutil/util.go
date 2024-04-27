package netutil

import (
	"net"
	"time"

	"go.f110.dev/xerrors"
)

func FindUnusedPort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, xerrors.WithStack(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	if err := l.Close(); err != nil {
		return -1, xerrors.WithStack(err)
	}

	return addr.Port, nil
}

func WaitListen(addr string, timeout time.Duration) error {
	sleepTime := time.Duration(timeout.Milliseconds() / 10)

	retry := 0
	for {
		if retry > 10 {
			return xerrors.Define("netutil: timed out").WithStack()
		}

		conn, err := net.DialTimeout("tcp", addr, 10*time.Millisecond)
		if err != nil {
			retry++
			time.Sleep(sleepTime * time.Millisecond)
			continue
		}
		conn.Close()
		break
	}

	return nil
}
