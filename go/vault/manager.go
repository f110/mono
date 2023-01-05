package vault

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"go.f110.dev/mono/go/netutil"
	"go.f110.dev/mono/go/stringsutil"
)

type ServerManager struct {
	bin       string
	port      int
	rootToken string
	cmd       *exec.Cmd
}

func NewServerManager(t *testing.T, binPath string) *ServerManager {
	port, err := netutil.FindUnusedPort()
	if err != nil {
		t.Fatal(err)
	}
	m := &ServerManager{bin: binPath, port: port, rootToken: stringsutil.RandomString(32)}
	m.start(t)
	t.Cleanup(func() {
		m.Stop()
	})

	return m
}

func (m *ServerManager) Addr() string {
	return fmt.Sprintf("http://127.0.0.1:%d", m.port)
}

func (m *ServerManager) Token() string {
	return m.rootToken
}

func (m *ServerManager) start(t *testing.T) {
	cmd := exec.Command(
		m.bin,
		"server",
		"-dev",
		fmt.Sprintf("-dev-listen-address=127.0.0.1:%d", m.port),
		fmt.Sprintf("-dev-root-token-id=%s", m.rootToken),
		"-dev-no-store-token",
	)
	if testing.Verbose() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start vault server: %v", err)
	}
	err := netutil.WaitListen(fmt.Sprintf(":%d", m.port), 3*time.Second)
	if err != nil {
		t.Fatalf("the vault server is not started within 3 seconds: %v", err)
	}
	m.cmd = cmd
}

func (m *ServerManager) Stop() {
	if m.cmd != nil {
		m.cmd.Process.Signal(syscall.SIGTERM)
	}
}
