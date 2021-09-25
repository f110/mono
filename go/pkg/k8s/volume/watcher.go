package volume

import (
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/xerrors"
)

type Watcher struct {
	watcher   *fsnotify.Watcher
	mountPath string
	fn        func()
}

func NewWatcher(mountPath string, fn func()) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}
	if err := watcher.Add(filepath.Join(mountPath, ".")); err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	w := &Watcher{watcher: watcher, mountPath: mountPath, fn: fn}
	go w.start()

	return w, nil
}

func (w *Watcher) start() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Create == fsnotify.Create {
				if event.Name == filepath.Join(w.mountPath, "..data") {
					w.fn()
				}
			}
		}
	}
}

func (w *Watcher) Stop() {
	w.watcher.Close()
}

func CanWatchVolume(path string) bool {
	if stat, err := os.Lstat(filepath.Join(path, "..data")); err == nil {
		if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			return true
		}
		return false
	}

	if stat, err := os.Lstat(filepath.Join(filepath.Dir(path), "..data")); err == nil {
		if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			return true
		}
		return false
	}

	return false
}

func FindMountPath(path string) (string, error) {
	if path[0] != '/' {
		return "", xerrors.New("k8s: path doesn't starting /")
	}

	p := path
	for {
		if _, err := os.Lstat(filepath.Join(p, "..data")); err == nil {
			return filepath.Join(p, "."), nil
		}
		s := filepath.Dir(p)
		if s == p {
			return "", xerrors.New("k8s: can not detect mount path")
		}
		p = s
	}
}
