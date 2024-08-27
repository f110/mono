package dbtestutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/netutil"
)

type TemporaryMySQL struct {
	Port int

	mysqldPath string
	baseDir    string
	cmd        *exec.Cmd
}

func NewTemporaryMySQL(ctx context.Context) (*TemporaryMySQL, error) {
	mysqldPath, err := exec.LookPath("mysqld")
	if err != nil {
		return nil, xerrors.Define("can't find mysqld").WithStack()
	}
	port, err := netutil.FindUnusedPort()
	if err != nil {
		return nil, err
	}

	baseDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range []string{"data", "secure"} {
		if err := os.Mkdir(filepath.Join(baseDir, v), 0755); err != nil {
			return nil, xerrors.WithStack(err)
		}
	}
	dataDir := filepath.Join(baseDir, "data")

	cmd := exec.CommandContext(ctx,
		"mysqld",
		"--initialize-insecure",
		"--user=mysql",
		"--datadir="+dataDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, xerrors.WithStack(err)
	}

	mysql := exec.CommandContext(context.Background(),
		"mysqld_safe",
		"--mysqld="+mysqldPath,
		"--user=mysql",
		"--basedir="+baseDir,
		"--datadir="+filepath.Join(baseDir, "data"),
		"--socket="+filepath.Join(baseDir, "mysqld.sock"),
		"--secure-file-priv="+filepath.Join(baseDir, "secure"),
		"--bind-address=127.0.0.1",
		fmt.Sprintf("--port=%d", port),
		"--skip-networking=0",
		fmt.Sprintf("--lc-messages-dir=%s", filepath.Clean(filepath.Join(filepath.Dir(mysqldPath), "../share/mysql8"))),
	)
	tempMySQL := &TemporaryMySQL{Port: port, mysqldPath: mysqldPath, baseDir: baseDir, cmd: mysql}
	runtime.SetFinalizer(tempMySQL, func(x *TemporaryMySQL) { x.Close() })
	return tempMySQL, nil
}

func (t *TemporaryMySQL) Start() error {
	if err := t.cmd.Start(); err != nil {
		logger.Log.Warn("Some error was occurred", logger.Error(err))
	}
	if err := netutil.WaitListen(fmt.Sprintf(":%d", t.Port), 10*time.Second); err != nil {
		return err
	}
	return nil
}

func (t *TemporaryMySQL) Close() {
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	os.RemoveAll(t.baseDir)
}

func (t *TemporaryMySQL) Verbose() {
	if t.cmd.Process != nil {
		return
	}
	t.cmd.Stdout = os.Stdout
	t.cmd.Stderr = os.Stderr
}

func CanUseTemporaryMySQL() bool {
	_, err := exec.LookPath("mysqld")
	if err != nil {
		return false
	}
	return true
}
