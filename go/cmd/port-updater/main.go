package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.f110.dev/xerrors"
	"golang.org/x/crypto/ripemd160"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/macports"
)

func portUpdater() error {
	var newVersion string
	var write bool
	cmd := &cli.Command{
		Use: "port-updater portfile",
		Run: func(ctx context.Context, _ *cli.Command, args []string) error {
			if len(args) != 2 {
				return xerrors.New("portfile path is required")
			}
			portfilePath := args[1]
			if newVersion == "" {
				return xerrors.New("--new-version is required")
			}

			f, err := os.Open(portfilePath)
			if err != nil {
				return xerrors.WithMessage(err, "failed to open portfile")
			}
			defer f.Close()
			tokens, err := macports.ParseAsTokens(f)
			if err != nil {
				return xerrors.WithMessage(err, "failed to parse portfile")
			}

			var isGolang, isRust bool
			for i := 0; i < len(tokens); i++ {
				v := tokens[i]
				if v.Type == macports.PortfileTokenIdent && v.Value == "PortGroup" {
					if i+1 < len(tokens) {
						next := tokens[i+1]
						if strings.HasPrefix(next.Value, "golang") {
							isGolang = true
						}
						if strings.HasPrefix(next.Value, "rust") || strings.HasPrefix(next.Value, "cargo") {
							isRust = true
						}
					}
				}
			}

			switch {
			case isGolang:
				tokens, err = updateGolang(ctx, http.DefaultClient, tokens, newVersion)
			case isRust:
				tokens, err = updateRust(ctx, http.DefaultClient, tokens, newVersion)
			default:
				return xerrors.New("unsupported port type: not golang or rust")
			}
			if err != nil {
				return err
			}

			out, err := macports.Output(tokens)
			if err != nil {
				return err
			}

			if write {
				return os.WriteFile(portfilePath, []byte(out), 0644)
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.Flags().String("new-version", "New version").Var(&newVersion)
	cmd.Flags().Bool("write", "Write changes back to file").Var(&write)
	return cmd.Execute(os.Args)
}

func findSetup(tokens []*macports.PortfileToken, key string) (tokenIndex int, repo, version, prefix string, found bool) {
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type == macports.PortfileTokenIdent && tokens[i].Value == key {
			if i+1 < len(tokens) {
				parts := strings.Split(tokens[i+1].Value, " ")
				switch key {
				case "github.setup":
					// github.setup owner repo version [prefix]
					if len(parts) >= 3 {
						repo = parts[0] + "/" + parts[1]
						version = parts[2]
					}
					if len(parts) > 3 {
						prefix = parts[3]
					}
				default:
					// go.setup github.com/owner/repo version [prefix]
					repo = parts[0]
					if len(parts) > 1 {
						version = parts[1]
					}
					if len(parts) > 2 {
						prefix = parts[2]
					}
				}
				return i, repo, version, prefix, true
			}
		}
	}
	return 0, "", "", "", false
}

func buildDownloadURL(setupKey, repo, version, prefix string) (string, error) {
	tag := prefix + version
	switch setupKey {
	case "go.setup":
		if strings.HasPrefix(repo, "github.com/") {
			return fmt.Sprintf("https://%s/archive/%s.tar.gz", repo, tag), nil
		}
	case "github.setup":
		return fmt.Sprintf("https://github.com/%s/archive/%s.tar.gz", repo, tag), nil
	}
	return "", fmt.Errorf("unsupported setup key %s with repo %s", setupKey, repo)
}

func downloadToTemp(ctx context.Context, agent *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	res, err := agent.Do(req)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", xerrors.Newf("download failed: %s (status %d)", url, res.StatusCode)
	}
	tmp, err := os.CreateTemp("", "port-updater-*.tar.gz")
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, res.Body); err != nil {
		os.Remove(tmp.Name())
		return "", xerrors.WithStack(err)
	}
	return tmp.Name(), nil
}

func computeChecksums(filePath string) (rmd160Hash, sha256Hash string, size int64, err error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", "", 0, xerrors.WithStack(err)
	}
	size = fi.Size()

	f, err := os.Open(filePath)
	if err != nil {
		return "", "", 0, xerrors.WithStack(err)
	}
	defer f.Close()
	hSha256 := sha256.New()
	hRmd160 := ripemd160.New()
	w := io.MultiWriter(hSha256, hRmd160)
	if _, err := io.Copy(w, f); err != nil {
		return "", "", 0, xerrors.WithStack(err)
	}
	sha256Hash = hex.EncodeToString(hSha256.Sum(nil))
	rmd160Hash = hex.EncodeToString(hRmd160.Sum(nil))

	return rmd160Hash, sha256Hash, size, nil
}

func effectiveVersion(newVersion, prefix string) string {
	if prefix == "v" && len(newVersion) > 0 && newVersion[0] == 'v' {
		return newVersion[1:]
	}
	return newVersion
}

func updateVersionToken(tokens []*macports.PortfileToken, setupKey, newVersion string) {
	for i := 0; i < len(tokens); i++ {
		v := tokens[i]
		if v.Type == macports.PortfileTokenIdent && v.Value == setupKey {
			n := tokens[i+1]
			parts := strings.Split(n.Value, " ")
			switch setupKey {
			case "github.setup":
				// github.setup owner repo version [prefix]
				owner, repo := parts[0], parts[1]
				prefix := ""
				if len(parts) > 3 {
					prefix = parts[3]
				}
				version := effectiveVersion(newVersion, prefix)
				if prefix != "" {
					n.Value = fmt.Sprintf("%s %s %s %s", owner, repo, version, prefix)
				} else {
					n.Value = fmt.Sprintf("%s %s %s", owner, repo, version)
				}
			default:
				// go.setup github.com/owner/repo version [prefix]
				repo := parts[0]
				prefix := ""
				if len(parts) > 2 {
					prefix = parts[2]
				}
				version := effectiveVersion(newVersion, prefix)
				if prefix != "" {
					n.Value = fmt.Sprintf("%s %s %s", repo, version, prefix)
				} else {
					n.Value = fmt.Sprintf("%s %s", repo, version)
				}
			}
			return
		}
	}
}

func updateChecksumTokens(tokens []*macports.PortfileToken, rmd160, sha256Hash string, size int64) {
	for i := 0; i < len(tokens); i++ {
		v := tokens[i]
		if v.Type != macports.PortfileTokenIdent || v.Value != "checksums" {
			continue
		}
		for j := i + 1; j < len(tokens); j++ {
			t := tokens[j]
			if t.Type != macports.PortfileTokenIdent {
				continue
			}
			if t.StartPos == 0 {
				break
			}
			fields := strings.Fields(t.Value)
			if len(fields) < 2 {
				continue
			}
			hasCont := strings.HasSuffix(t.Value, " \\")
			suffix := ""
			if hasCont {
				suffix = " \\"
			}
			switch fields[0] {
			case "rmd160":
				t.Value = fmt.Sprintf("rmd160  %s%s", rmd160, suffix)
			case "sha256":
				t.Value = fmt.Sprintf("sha256  %s%s", sha256Hash, suffix)
			case "size":
				t.Value = fmt.Sprintf("size    %d%s", size, suffix)
			}
		}
		break
	}
}

func updateGolang(ctx context.Context, agent *http.Client, tokens []*macports.PortfileToken, newVersion string) ([]*macports.PortfileToken, error) {
	_, repo, _, prefix, found := findSetup(tokens, "go.setup")
	if !found {
		return nil, fmt.Errorf("go.setup not found in portfile")
	}

	version := effectiveVersion(newVersion, prefix)
	url, err := buildDownloadURL("go.setup", repo, version, prefix)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "Downloading %s\n", url)
	tmpFile, err := downloadToTemp(ctx, agent, url)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to download source tarball")
	}
	defer os.Remove(tmpFile)

	rmd160, sha256Hash, size, err := computeChecksums(tmpFile)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to compute checksums")
	}

	updateVersionToken(tokens, "go.setup", newVersion)
	updateChecksumTokens(tokens, rmd160, sha256Hash, size)

	return tokens, nil
}

func updateRust(ctx context.Context, agent *http.Client, tokens []*macports.PortfileToken, newVersion string) ([]*macports.PortfileToken, error) {
	var setupKey, repo, prefix string
	var found bool
	for _, key := range []string{"github.setup", "cargo.setup"} {
		_, repo, _, prefix, found = findSetup(tokens, key)
		if found {
			setupKey = key
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("setup command not found in portfile")
	}

	version := effectiveVersion(newVersion, prefix)
	url, err := buildDownloadURL(setupKey, repo, version, prefix)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "Downloading %s\n", url)
	tmpFile, err := downloadToTemp(ctx, agent, url)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to download source tarball")
	}
	defer os.Remove(tmpFile)

	rmd160, sha256Hash, size, err := computeChecksums(tmpFile)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to compute checksums")
	}

	updateVersionToken(tokens, setupKey, newVersion)
	updateChecksumTokens(tokens, rmd160, sha256Hash, size)

	tokens, err = updateCargoCrates(tokens, tmpFile)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to update cargo.crates")
	}

	return tokens, nil
}

func updateCargoCrates(tokens []*macports.PortfileToken, tarballPath string) ([]*macports.PortfileToken, error) {
	tmpDir, err := os.MkdirTemp("", "port-updater-extract-*")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("tar", "xzf", tarballPath, "-C", tmpDir).Run(); err != nil {
		return nil, xerrors.WithMessage(err, "failed to extract tarball")
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no files in extracted tarball")
	}
	srcDir := filepath.Join(tmpDir, entries[0].Name())
	cargoLock := filepath.Join(srcDir, "Cargo.lock")
	if _, err := os.Stat(cargoLock); err != nil {
		return nil, xerrors.WithMessage(err, "Cargo.lock not found in extracted source")
	}

	fmt.Fprintf(os.Stderr, "Running cargo2port %s\n", cargoLock)
	out, err := exec.Command("cargo2port", cargoLock).Output()
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to run cargo2port")
	}

	cratesOutput := strings.TrimSpace(string(out))
	if cratesOutput == "" {
		return tokens, nil
	}

	// Parse cargo2port output as tokens (append \n so parser doesn't lose the last token at EOF)
	cargoTokens, err := macports.ParseAsTokens(strings.NewReader(cratesOutput + "\n"))
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to parse cargo2port output")
	}
	// Remove trailing linebreak tokens
	for len(cargoTokens) > 0 && cargoTokens[len(cargoTokens)-1].Type == macports.PortfileTokenLineBreak {
		cargoTokens = cargoTokens[:len(cargoTokens)-1]
	}

	// Find cargo.crates key and value range in original tokens [keyIdx, valEnd)
	keyIdx := -1
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type == macports.PortfileTokenIdent && tokens[i].Value == "cargo.crates" {
			keyIdx = i
			break
		}
	}
	if keyIdx < 0 {
		return tokens, nil
	}
	valEnd := keyIdx + 2 // key + at least one value token
	for j := keyIdx + 2; j < len(tokens); j++ {
		if tokens[j].Type == macports.PortfileTokenLineBreak {
			if j+1 < len(tokens) && tokens[j+1].Type == macports.PortfileTokenIdent && tokens[j+1].StartPos > 0 {
				valEnd = j + 2
				j++
				continue
			}
			break
		}
	}

	// Replace original cargo.crates tokens with parsed cargo2port tokens
	result := make([]*macports.PortfileToken, 0, len(tokens)-valEnd+keyIdx+len(cargoTokens))
	result = append(result, tokens[:keyIdx]...)
	result = append(result, cargoTokens...)
	result = append(result, tokens[valEnd:]...)

	return result, nil
}

func main() {
	if err := portUpdater(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
