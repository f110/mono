package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	"golang.org/x/term"

	"go.f110.dev/mono/go/cli"
)

const (
	anthropicAPIBase  = "https://api.anthropic.com"
	anthropicVersion  = "2023-06-01"
	promptSentinel    = "__prompt__"
	assistedByKey     = "Assisted-by"
	defaultAgentLabel = "Claude"
)

type anthropicModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type anthropicModelsResponse struct {
	Data []anthropicModel `json:"data"`
}

type trailerSpec struct {
	key          string
	resolveValue func(ctx context.Context, noCache bool) (string, error)
}

func supportedTrailers() []trailerSpec {
	return []trailerSpec{
		{key: assistedByKey, resolveValue: pickAssistedByValue},
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var rev string
	var assistedBy string
	var noCache bool
	var rawTrailers []string

	cmd := &cli.Command{
		Use:   "jj-trailer",
		Short: "Add trailers to a jj commit",
	}

	assistedByFlag := cmd.Flags().String("assisted-by", "Add Assisted-by trailer. With no value, prompt to pick a model from the Anthropic API").Default("").Var(&assistedBy)
	assistedByFlag.Flag().NoOptDefVal = promptSentinel
	cmd.Flags().String("revision", "Revision to add the trailer to").Shorthand("r").Default("@").Var(&rev)
	cmd.Flags().Bool("no-cache", "Skip the cached models list and refetch from the Anthropic API").Default(false).Var(&noCache)
	trailerFlag := cmd.Flags().StringArray("trailer", "Add a raw trailer in 'Key=Value' form. May be repeated. Duplicates of the same key are allowed").Shorthand("t").Var(&rawTrailers)

	cmd.Run = func(ctx context.Context, _ *cli.Command, _ []string) error {
		desc, err := readDescription(ctx, rev)
		if err != nil {
			return err
		}

		rawAdds, err := parseRawTrailers(rawTrailers)
		if err != nil {
			return err
		}

		newDesc := desc
		changed := false

		switch {
		case assistedByFlag.Flag().Changed:
			next, ok, err := applyAssistedBy(ctx, newDesc, assistedBy, noCache)
			if err != nil {
				return err
			}
			newDesc = next
			changed = changed || ok
		case !trailerFlag.Flag().Changed:
			spec, err := promptTrailer(supportedTrailers())
			if err != nil {
				return err
			}
			if hasTrailer(newDesc, spec.key) {
				fmt.Fprintf(os.Stderr, "%s: already present, nothing to do\n", spec.key)
			} else {
				value, err := spec.resolveValue(ctx, noCache)
				if err != nil {
					return err
				}
				newDesc, _ = addTrailer(newDesc, spec.key, value)
				changed = true
			}
		}

		for _, kv := range rawAdds {
			newDesc = appendTrailer(newDesc, kv.Key, kv.Value)
			changed = true
		}

		if !changed {
			return nil
		}
		return writeDescription(ctx, rev, newDesc)
	}

	return cmd.Execute(os.Args)
}

type keyValue struct {
	Key   string
	Value string
}

func parseRawTrailers(in []string) ([]keyValue, error) {
	out := make([]keyValue, 0, len(in))
	for _, s := range in {
		k, v, ok := strings.Cut(s, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return nil, xerrors.Definef("invalid --trailer %q: expected Key=Value", s).WithStack()
		}
		out = append(out, keyValue{Key: k, Value: v})
	}
	return out, nil
}

func applyAssistedBy(ctx context.Context, desc, explicit string, noCache bool) (string, bool, error) {
	if hasTrailer(desc, assistedByKey) {
		fmt.Fprintf(os.Stderr, "%s: already present, nothing to do\n", assistedByKey)
		return desc, false, nil
	}
	value := explicit
	if value == "" || value == promptSentinel {
		v, err := pickAssistedByValue(ctx, noCache)
		if err != nil {
			return desc, false, err
		}
		value = v
	}
	next, _ := addTrailer(desc, assistedByKey, value)
	return next, true, nil
}

func readDescription(ctx context.Context, rev string) (string, error) {
	c := exec.CommandContext(ctx, "jj", "log", "-r", rev, "--no-graph", "--template", `json(description) ++ "\n"`)
	buf, err := c.Output()
	if err != nil {
		return "", xerrors.WithMessage(err, "jj log failed")
	}
	var desc string
	if err := json.Unmarshal(bytes.TrimSpace(buf), &desc); err != nil {
		return "", xerrors.WithStack(err)
	}
	return desc, nil
}

func writeDescription(ctx context.Context, rev, desc string) error {
	c := exec.CommandContext(ctx, "jj", "describe", "-r", rev, "--stdin")
	c.Stdin = strings.NewReader(desc)
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return xerrors.WithMessage(err, "jj describe failed")
	}
	return nil
}

func pickAssistedByValue(ctx context.Context, noCache bool) (string, error) {
	cachePath, err := defaultCachePath()
	if err != nil {
		return "", err
	}

	var models []anthropicModel
	if !noCache {
		cached, err := loadModelsCache(cachePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to read models cache: %v\n", err)
		}
		models = cached
	}

	if len(models) == 0 {
		apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		if apiKey == "" {
			return "", xerrors.New("ANTHROPIC_API_KEY is not set; pass --assisted-by=AGENT:MODEL explicitly")
		}
		fetched, err := fetchModels(ctx, apiKey)
		if err != nil {
			return "", err
		}
		if len(fetched) == 0 {
			return "", xerrors.New("no models returned from Anthropic API")
		}
		if err := writeModelsCache(cachePath, fetched); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write models cache: %v\n", err)
		}
		models = fetched
	}

	m, err := promptModel(models)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", defaultAgentLabel, m.ID), nil
}

func fetchModels(ctx context.Context, apiKey string) ([]anthropicModel, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anthropicAPIBase+"/v1/models?limit=100", nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	req.Header.Set("anthropic-version", anthropicVersion)
	if strings.HasPrefix(apiKey, "sk-ant-oat") {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, xerrors.Definef("anthropic api: %s: %s", resp.Status, strings.TrimSpace(string(body))).WithStack()
	}

	var out anthropicModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return out.Data, nil
}

func promptModel(models []anthropicModel) (anthropicModel, error) {
	labels := make([]string, len(models))
	for i, m := range models {
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		labels[i] = fmt.Sprintf("%s (%s)", name, m.ID)
	}
	idx, err := selectInteractive("Select a model:", labels)
	if err != nil {
		return anthropicModel{}, err
	}
	return models[idx], nil
}

func promptTrailer(specs []trailerSpec) (trailerSpec, error) {
	labels := make([]string, len(specs))
	for i, t := range specs {
		labels[i] = t.key
	}
	idx, err := selectInteractive("Select a trailer:", labels)
	if err != nil {
		return trailerSpec{}, err
	}
	return specs[idx], nil
}

type keyAction int

const (
	actionNone keyAction = iota
	actionUp
	actionDown
	actionConfirm
	actionAbort
)

// decodeKey maps a raw input byte sequence to a navigation action.
func decodeKey(b []byte) keyAction {
	n := len(b)
	switch {
	case n >= 3 && b[0] == 0x1b && b[1] == '[' && b[2] == 'A':
		return actionUp
	case n >= 3 && b[0] == 0x1b && b[1] == '[' && b[2] == 'B':
		return actionDown
	case n == 1 && (b[0] == 'k' || b[0] == 0x10): // k / Ctrl-P
		return actionUp
	case n == 1 && (b[0] == 'j' || b[0] == 0x0e): // j / Ctrl-N
		return actionDown
	case n == 1 && (b[0] == '\r' || b[0] == '\n'):
		return actionConfirm
	case n == 1 && (b[0] == 0x03 || b[0] == 0x1b || b[0] == 'q'):
		return actionAbort
	}
	return actionNone
}

// selectInteractive renders a vertical list on stderr and lets the user move
// with arrow keys (also j/k, Ctrl-N/Ctrl-P) and confirm with Enter. Returns
// the chosen index. Aborts on Ctrl-C, ESC, or q.
//
// Stdin must be a terminal; otherwise an error is returned.
func selectInteractive(prompt string, items []string) (int, error) {
	if len(items) == 0 {
		return 0, xerrors.New("no items to select from")
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return 0, xerrors.New("stdin is not a terminal")
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return 0, xerrors.WithStack(err)
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	w := os.Stderr
	fmt.Fprint(w, "\x1b[?25l")
	defer fmt.Fprint(w, "\x1b[?25h")

	const helpLine = "\x1b[2m↑↓/jk/^P^N: move  Enter: select  q/^C: cancel\x1b[0m"
	totalLines := len(items) + 2 // prompt + items + help

	selected := 0
	render := func() {
		fmt.Fprintf(w, "%s\r\n", prompt)
		for i, item := range items {
			if i == selected {
				fmt.Fprintf(w, "\x1b[7m> %s\x1b[0m\r\n", item)
			} else {
				fmt.Fprintf(w, "  %s\r\n", item)
			}
		}
		fmt.Fprintf(w, "%s\r\n", helpLine)
	}
	clear := func() {
		for i := 0; i < totalLines; i++ {
			fmt.Fprint(w, "\x1b[1A\x1b[2K")
		}
		fmt.Fprint(w, "\r")
	}

	render()

	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			if err == io.EOF {
				clear()
				return 0, xerrors.New("aborted")
			}
			clear()
			return 0, xerrors.WithStack(err)
		}
		switch decodeKey(buf[:n]) {
		case actionUp:
			if selected > 0 {
				selected--
			}
		case actionDown:
			if selected < len(items)-1 {
				selected++
			}
		case actionConfirm:
			clear()
			return selected, nil
		case actionAbort:
			clear()
			return 0, xerrors.New("aborted")
		default:
			continue
		}
		clear()
		render()
	}
}
