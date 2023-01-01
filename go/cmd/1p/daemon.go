package main

import (
	"context"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/golang/protobuf/ptypes"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/clipboard"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/opvault"
)

const (
	socketFilename = "vault.sock"
	configFilename = "1p.conf"
)

type vaultConfig struct {
	VaultPath string `yaml:"vaultPath"`
}

type vault struct {
	server         *grpc.Server
	config         *vaultConfig
	configFilePath string
	reader         *opvault.Reader
	clipboard      *clipboard.Clipboard
}

func NewVault() (*vault, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	confDir := filepath.Join(homeDir, ConfigDirName)
	if _, err := os.Stat(confDir); os.IsNotExist(err) {
		if err := os.Mkdir(confDir, 0700); err != nil {
			return nil, xerrors.WithStack(err)
		}
	}

	cb, err := clipboard.New()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	v := &vault{
		configFilePath: filepath.Join(confDir, configFilename),
		clipboard:      cb,
	}
	if err := v.readConfig(v.configFilePath); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if v.config.VaultPath != "" {
		v.reader = opvault.NewReader(v.config.VaultPath)
	}

	listener, err := net.Listen("unix", filepath.Join(confDir, socketFilename))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	v.server = grpc.NewServer()
	RegisterOnePasswordServer(v.server, v)
	go func() {
		err := v.server.Serve(listener)
		if err != nil {
			logger.Log.Error("Failed serve grpc", zap.Error(err))
		}
	}()

	return v, nil
}

func (v *vault) readConfig(path string) error {
	buf, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		v.config = &vaultConfig{}
		return nil
	}
	conf := &vaultConfig{}
	if err := yaml.Unmarshal(buf, conf); err != nil {
		return xerrors.WithStack(err)
	}
	v.config = conf

	return nil
}

func (v *vault) persistConfig() error {
	buf, err := yaml.Marshal(v.config)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := ioutil.WriteFile(v.configFilePath, buf, 0644); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (v *vault) Shutdown() {
	v.server.GracefulStop()
}

func (v *vault) Unlock(_ context.Context, req *RequestUnlock) (*ResponseUnlock, error) {
	if v.reader == nil {
		return nil, xerrors.New("opvault is not opened")
	}
	if err := v.reader.Unlock(string(req.MasterPassword)); err != nil {
		return nil, err
	}
	_ = v.reader.Items()
	if errors.Is(v.reader.Err(), opvault.ErrInvalidData) {
		v.reader.Lock()
		return &ResponseUnlock{Success: false}, v.reader.Err()
	}

	return &ResponseUnlock{Success: true}, nil
}

func (v *vault) Lock(_ context.Context, _ *RequestLock) (*ResponseLock, error) {
	if v.reader == nil {
		return nil, xerrors.New("opvault is not opened")
	}
	v.reader.Lock()

	return &ResponseLock{}, nil
}

func (v *vault) UseVault(_ context.Context, useVault *RequestUseVault) (*ResponseUseVault, error) {
	if useVault.Path == v.config.VaultPath {
		return &ResponseUseVault{}, nil
	}
	v.config.VaultPath = useVault.Path
	v.reader = opvault.NewReader(useVault.Path)
	if err := v.persistConfig(); err != nil {
		return nil, err
	}

	return &ResponseUseVault{}, nil
}

func (v *vault) Info(_ context.Context, _ *RequestInfo) (*ResponseInfo, error) {
	locked, err := v.reader.IsLocked()
	if err != nil {
		return nil, err
	}
	return &ResponseInfo{
		Path:   v.config.VaultPath,
		Locked: locked,
	}, nil
}

func (v *vault) List(_ context.Context, _ *RequestList) (*ResponseList, error) {
	locked, err := v.reader.IsLocked()
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, xerrors.New("Vault is locked. You have to unlock first.")
	}

	items := v.reader.Items()
	if err := v.reader.Err(); err != nil {
		return nil, err
	}

	result := make([]*Item, 0)
	for _, v := range items {
		item, err := toItem(v)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return &ResponseList{
		Items: result,
	}, nil
}

func (v *vault) Get(_ context.Context, req *RequestGet) (*ResponseGet, error) {
	locked, err := v.reader.IsLocked()
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, xerrors.New("Vault is locked. You have to unlock first")
	}

	items := v.reader.Items()
	if err := v.reader.Err(); err != nil {
		return nil, err
	}
	k, ok := items[req.Uuid]
	if !ok {
		return nil, xerrors.Newf("item not found: %s", req.Uuid)
	}
	item, err := toItem(k)
	if err != nil {
		return nil, err
	}

	return &ResponseGet{Item: item}, nil
}

func (v *vault) SetClipboard(_ context.Context, req *RequestSetClipboard) (*ResponseSetClipboard, error) {
	locked, err := v.reader.IsLocked()
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, xerrors.New("Vault is locked. You have to unlock first")
	}

	items := v.reader.Items()
	if err := v.reader.Err(); err != nil {
		return nil, err
	}
	k, ok := items[req.Uuid]
	if !ok {
		return nil, xerrors.Newf("item not found: %s", req.Uuid)
	}

	password := ""
	if k.Detail != nil {
		for _, v := range k.Detail.Fields {
			if v.Type == "P" {
				password = v.Value
			}
		}
	}
	v.clipboard.Set(password)

	return &ResponseSetClipboard{}, nil
}

func toItem(in *opvault.Item) (*Item, error) {
	createdAt, err := ptypes.TimestampProto(in.Created)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	updatedAt, err := ptypes.TimestampProto(in.Updated)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	password := ""
	if in.Detail != nil {
		for _, v := range in.Detail.Fields {
			if v.Type == "P" {
				password = v.Value
			}
		}
	}

	return &Item{
		Uuid:      in.UUID,
		Category:  string(in.Category),
		Title:     in.Overview.Title,
		Url:       in.Overview.URL,
		Password:  password,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

var _ OnePasswordServer = &vault{}

type daemon struct {
	*fsm.FSM

	vault *vault
}

const (
	stateInit fsm.State = iota
	stateStart
	stateShutdown
)

func NewDaemon() *daemon {
	d := &daemon{}
	d.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:     d.init,
			stateStart:    d.start,
			stateShutdown: d.shutdown,
		},
		stateInit,
		stateShutdown,
	)
	d.SignalHandling(os.Interrupt, syscall.SIGTERM)

	return d
}

func (d *daemon) init() (fsm.State, error) {
	v, err := NewVault()
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	d.vault = v

	if _, err := syscall.Setsid(); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	return stateStart, nil
}

func (d *daemon) start() (fsm.State, error) {
	go func() {
		<-time.After(3 * time.Hour)
		d.Shutdown()
	}()

	return fsm.WaitState, nil
}

func (d *daemon) shutdown() (fsm.State, error) {
	d.vault.Shutdown()
	return fsm.CloseState, nil
}
