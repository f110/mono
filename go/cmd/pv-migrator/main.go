package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

const (
	lockFilename = "pv-migrate-completed"
)

func copyFile(from, to string) error {
	f, err := os.Open(from)
	if err != nil {
		return xerrors.WithStack(err)
	}
	s, err := os.Stat(from)
	if err != nil {
		return xerrors.WithStack(err)
	}
	st, ok := s.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("could not convert to syscall.Stat_t: %v", from)
	}

	d, err := os.Create(to)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if _, err = io.Copy(d, f); err != nil {
		return xerrors.WithStack(err)
	}
	if err := f.Close(); err != nil {
		return xerrors.WithStack(err)
	}
	if err := d.Close(); err != nil {
		return xerrors.WithStack(err)
	}

	if err := os.Chmod(to, s.Mode()); err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.Chown(to, int(st.Uid), int(st.Gid)); err != nil {
		return xerrors.WithStack(err)
	}
	logger.Log.Info("Copy file", zap.String("from", from), zap.String("to", to), zap.Uint32("mode", uint32(s.Mode())), zap.Int("uid", int(st.Uid)), zap.Int("gid", int(st.Gid)))

	return nil
}

func copyDirectory(from, to string) error {
	logger.Log.Info("Copy directory", zap.String("from", from), zap.String("to", to))
	entries, err := os.ReadDir(from)
	if err != nil {
		return xerrors.WithStack(err)
	}

	for _, v := range entries {
		t := filepath.Join(to, filepath.Base(v.Name()))
		i, err := v.Info()
		if err != nil {
			return xerrors.WithStack(err)
		}

		if v.IsDir() {
			st, ok := i.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("could not convert syscall.Stat_t: %v", v.Name())
			}

			if err := os.MkdirAll(t, i.Mode()); err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.Chmod(t, i.Mode()); err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.Chown(t, int(st.Uid), int(st.Gid)); err != nil {
				return xerrors.WithStack(err)
			}
			if err := copyDirectory(filepath.Join(from, v.Name()), t); err != nil {
				return err
			}
			continue
		}

		if i.Mode()&os.ModeSymlink == os.ModeSymlink {
			continue
		}

		if err := copyFile(filepath.Join(from, v.Name()), t); err != nil {
			return err
		}
	}

	return nil
}

func migrateDirectory(from, to string) error {
	to = filepath.Clean(to)
	if _, err := os.Stat(filepath.Join(to, lockFilename)); !os.IsNotExist(err) {
		b, err := os.ReadFile(filepath.Join(to, lockFilename))
		if err != nil {
			return xerrors.WithStack(err)
		}

		logger.Log.Info("Already migrated", zap.String("locked_at", string(b)))
		return nil
	}
	from = filepath.Clean(from)

	if err := copyDirectory(from, to); err != nil {
		return err
	}

	// Write lock file
	return os.WriteFile(filepath.Join(to, lockFilename), []byte(time.Now().Format(time.RFC3339)), 0644)
}

func main() {
	logger.Init()

	sourceDir := ""
	destinationDir := ""
	fs := pflag.NewFlagSet("pv-migrator", pflag.PanicOnError)
	fs.StringVarP(&sourceDir, "source", "s", sourceDir, "Source directory")
	fs.StringVarP(&destinationDir, "destination", "d", destinationDir, "Destination directory")
	fs.Parse(os.Args)

	if err := migrateDirectory(sourceDir, destinationDir); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
