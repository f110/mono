package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/peco/peco"
	"github.com/shirou/gopsutil/v3/process"
	"go.f110.dev/xerrors"
	"golang.org/x/term"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/opvault"
)

const (
	ConfigDirName = ".1p"
)

var subcommands = []func(command *cli.Command){
	Daemon,
	Shutdown,
	UseVault,
	Info,
	Unlock,
	List,
	Get,
}

func Main() error {
	client, err := dial()
	if err != nil && err == ErrDaemonNotExist {
		cmd := exec.Command(os.Args[0], "daemon")
		if err := cmd.Run(); err != nil {
			return xerrors.WithStack(err)
		}
		time.Sleep(100 * time.Millisecond)
		client, err = dial()
		if err != nil {
			return xerrors.WithStack(err)
		}
	} else if err != nil {
		return xerrors.WithStack(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	info, err := client.Info(ctx, &RequestInfo{})
	cancel()
	if err != nil {
		return xerrors.WithStack(err)
	}
	if info.Locked {
		cmd := exec.Command(os.Args[0], "unlock")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return xerrors.WithStack(err)
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	list, err := client.List(ctx, &RequestList{})
	cancel()
	if err != nil {
		return xerrors.WithStack(err)
	}
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].Title < list.Items[j].Title
	})
	input := new(bytes.Buffer)
	for _, v := range list.Items {
		if opvault.Category(v.Category) != opvault.CategoryLogin {
			continue
		}
		fmt.Fprintf(input, "%s %s\n", v.Uuid, v.Title)
	}
	selector := peco.New()
	selector.Stdin = input
	selector.Run(context.Background())

	selected, err := selector.CurrentLineBuffer().LineAt(selector.Location().LineNumber())
	if err != nil {
		return xerrors.WithStack(err)
	}

	s := strings.SplitN(selected.DisplayString(), " ", 2)
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	_, err = client.SetClipboard(ctx, &RequestSetClipboard{Uuid: s[0]})
	cancel()
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func Daemon(rootCmd *cli.Command) {
	daemonize := false
	foreground := false
	daemonCmd := &cli.Command{
		Use: "daemon",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			if !foreground {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return xerrors.WithStack(err)
				}
				socketFile := filepath.Join(homeDir, ConfigDirName, socketFilename)
				if _, err := os.Stat(socketFile); !os.IsNotExist(err) {
					_, err := dial()
					if err != nil {
						os.Remove(socketFile)
					} else {
						// Already running
						logger.Log.Debug("The daemon is already running")
						return nil
					}
				}

				if !daemonize {
					cmd := exec.Command(os.Args[0], "daemon", "--daemonize")
					if err := cmd.Start(); err != nil {
						return xerrors.WithStack(err)
					}
					pid := cmd.Process.Pid
					if err := ioutil.WriteFile(filepath.Join(homeDir, ConfigDirName, "1p.pid"), []byte(strconv.Itoa(pid)), 0644); err != nil {
						return xerrors.WithStack(err)
					}
					return nil
				}

				defer func() {
					os.Remove(filepath.Join(homeDir, ConfigDirName, "1p.pid"))
				}()
			}

			d := NewDaemon()
			return d.Loop()
		},
	}
	daemonCmd.Flags().Bool("daemonize", "Daemonize").Var(&daemonize)
	daemonCmd.Flags().Bool("foreground", "").Var(&foreground)

	rootCmd.AddCommand(daemonCmd)
}

func Shutdown(rootCmd *cli.Command) {
	shutdownCmd := &cli.Command{
		Use: "shutdown",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return xerrors.WithStack(err)
			}
			pidFile := filepath.Join(homeDir, ConfigDirName, "1p.pid")
			if _, err := os.Stat(pidFile); os.IsNotExist(err) {
				return nil
			}

			buf, err := ioutil.ReadFile(pidFile)
			if err != nil {
				return xerrors.WithStack(err)
			}
			pid, err := strconv.Atoi(string(buf))
			if err != nil {
				os.Remove(pidFile)
				return xerrors.WithStack(err)
			}
			if exists, err := process.PidExists(int32(pid)); err != nil {
				return xerrors.WithStack(err)
			} else if exists {
				proc, err := os.FindProcess(pid)
				if err != nil {
					return xerrors.WithStack(err)
				}
				proc.Signal(syscall.SIGTERM)
			} else {
				os.Remove(pidFile)
			}

			return nil
		},
	}
	rootCmd.AddCommand(shutdownCmd)
}

func UseVault(rootCmd *cli.Command) {
	path := ""
	useVaultCmd := &cli.Command{
		Use: "use-vault",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			if path == "" {
				return xerrors.New("--path is mandatory")
			}

			client, err := dial()
			if err != nil {
				return xerrors.WithStack(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			_, err = client.UseVault(ctx, &RequestUseVault{Path: path})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}

			return nil
		},
	}
	useVaultCmd.Flags().String("path", "The path to opvault").Var(&path)

	rootCmd.AddCommand(useVaultCmd)
}

func Info(rootCmd *cli.Command) {
	infoCmd := &cli.Command{
		Use: "info",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			client, err := dial()
			if err != nil {
				return xerrors.WithStack(err)
			}
			tCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			res, err := client.Info(tCtx, &RequestInfo{})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			fmt.Fprintf(os.Stdout, "Current opvault is %s\n", res.Path)
			if res.Locked {
				fmt.Fprintln(os.Stdout, "Vault is Locked")
			} else {
				fmt.Fprintln(os.Stdout, "Vault is Unlocked")
			}

			return nil
		},
	}

	rootCmd.AddCommand(infoCmd)
}

func Unlock(rootCmd *cli.Command) {
	unlockCmd := &cli.Command{
		Use: "unlock",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			client, err := dial()
			if err != nil {
				return xerrors.WithStack(err)
			}
			tCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			info, err := client.Info(tCtx, &RequestInfo{})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			if !info.Locked {
				fmt.Fprintln(os.Stdout, "Already unlocked")
				return nil
			}

			fmt.Printf("Master passowrd: ")
			masterPassword, err := term.ReadPassword(syscall.Stdin)
			if err != nil {
				return xerrors.WithStack(err)
			}
			fmt.Println()
			tCtx, cancel = context.WithTimeout(ctx, 1*time.Second)
			res, err := client.Unlock(tCtx, &RequestUnlock{MasterPassword: masterPassword})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			if !res.Success {
				return xerrors.New("unlock failed.")
			}
			fmt.Println("Unlock succeeded")

			return nil
		},
	}

	rootCmd.AddCommand(unlockCmd)
}

func List(rootCmd *cli.Command) {
	listCmd := &cli.Command{
		Use: "list",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			client, err := dial()
			if err != nil {
				return xerrors.WithStack(err)
			}
			tCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			info, err := client.Info(tCtx, &RequestInfo{})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			if info.Locked {
				return xerrors.New("Vault is locked")
			}

			tCtx, cancel = context.WithTimeout(ctx, 1*time.Second)
			list, err := client.List(tCtx, &RequestList{})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			sort.Slice(list.Items, func(i, j int) bool {
				return list.Items[i].Title < list.Items[j].Title
			})
			for _, v := range list.Items {
				fmt.Printf("%s %s\n", v.Uuid, v.Title)
			}

			return nil
		},
	}

	rootCmd.AddCommand(listCmd)
}

func Get(rootCmd *cli.Command) {
	getCmd := &cli.Command{
		Use: "get UUID",
		Run: func(ctx context.Context, _ *cli.Command, args []string) error {
			if len(args) != 1 {
				return xerrors.New("UUID is required")
			}
			client, err := dial()
			if err != nil {
				return xerrors.WithStack(err)
			}
			tCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			info, err := client.Info(tCtx, &RequestInfo{})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			if info.Locked {
				return xerrors.New("Vault is locked")
			}

			tCtx, cancel = context.WithTimeout(ctx, 1*time.Second)
			res, err := client.Get(tCtx, &RequestGet{Uuid: args[0]})
			cancel()
			if err != nil {
				return xerrors.WithStack(err)
			}
			fmt.Printf("UUID: %s\n", res.Item.Uuid)
			fmt.Printf("Title: %s\n", res.Item.Title)
			fmt.Printf("Password: %s\n", res.Item.Password)

			return nil
		},
	}

	rootCmd.AddCommand(getCmd)
}

var ErrDaemonNotExist = xerrors.New("daemon not exist")

func dial() (OnePasswordClient, error) {
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
	pidFile := filepath.Join(homeDir, ConfigDirName, "1p.pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return nil, ErrDaemonNotExist
	}

	buf, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	pid, err := strconv.Atoi(string(buf))
	if err != nil {
		os.Remove(pidFile)
		return nil, ErrDaemonNotExist
	}
	if exists, err := process.PidExists(int32(pid)); err != nil || !exists {
		return nil, ErrDaemonNotExist
	}

	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", addr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	conn, err := grpc.DialContext(
		ctx,
		filepath.Join(confDir, socketFilename),
		grpc.WithInsecure(),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
	cancel()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return NewOnePasswordClient(conn), nil
}

func AddCommand(rootCmd *cli.Command) {
	for _, v := range subcommands {
		v(rootCmd)
	}
}
