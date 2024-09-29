package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/jma"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/prometheus/exporter"
	"go.f110.dev/mono/go/ucl"
)

type command struct {
	*fsm.FSM

	confFile string

	conf     *configuration
	exporter *exporter.Amedas
	s        *http.Server
}

const (
	stateInit fsm.State = iota
	stateStartServer
	stateShutdown
)

func newCommand() *command {
	c := &command{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:        c.init,
			stateStartServer: c.startServer,
			stateShutdown:    c.shutdown,
		},
		stateInit,
		stateShutdown,
	)
	c.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 10*time.Second)
	}
	c.FSM.DisableErrorOutput = true
	return c
}

func (c *command) init(ctx context.Context) (fsm.State, error) {
	f, err := os.Open(c.confFile)
	if err != nil {
		return fsm.Error(err)
	}
	buf, err := ucl.NewDecoder(f).ToJSON(nil)
	if err != nil {
		return fsm.Error(err)
	}
	var conf configuration
	if err := json.Unmarshal(buf, &conf); err != nil {
		return fsm.Error(err)
	}
	c.conf = &conf

	var targets [][]int
	for _, site := range conf.AmedasSite.Sites {
		var prefNo int
		switch site.Pref {
		case "tokyo":
			prefNo = int(jma.Tokyo)
		case "chiba":
			prefNo = int(jma.Chiba)
		}
		targets = append(targets, []int{prefNo, site.No})
	}
	c.exporter = exporter.NewAmedasExporter(targets)
	return fsm.Next(stateStartServer)
}

func (c *command) startServer(_ context.Context) (fsm.State, error) {
	server := &http.Server{
		Addr: c.conf.Exporter.Addr,
	}

	registerer := prometheus.NewRegistry()
	registerer.MustRegister(c.exporter)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.InstrumentMetricHandler(registerer, promhttp.HandlerFor(registerer, promhttp.HandlerOpts{})))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html>
 <head><title>Amedas Exporter</title></head>
 <body>
 <h1>Amedas Exporter</h1>
 <p><a href='/metrics'>Metrics</a></p>
 </body>
 </html>`)
	})
	server.Handler = mux

	c.s = server
	go func() {
		logger.Log.Info("Start amedas exporter", zap.String("addr", c.s.Addr))
		if err := c.s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("http server error", logger.Error(err))
		}
	}()
	return fsm.Wait()
}

func (c *command) shutdown(ctx context.Context) (fsm.State, error) {
	if c.s != nil {
		if err := c.s.Shutdown(ctx); err != nil {
			return fsm.Error(err)
		}
	}
	return fsm.Finish()
}

func (c *command) Flags(fs *cli.FlagSet) {
	fs.String("conf", "Configuration file path").Var(&c.confFile).Required()
}

func amedasExporter() error {
	c := newCommand()
	cmd := &cli.Command{
		Use: "amedas-exporter",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.LoopContext(ctx)
		},
	}
	c.Flags(cmd.Flags())
	return cmd.Execute(os.Args)
}

func main() {
	if err := amedasExporter(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

type configuration struct {
	Exporter   *configurationExporter   `json:"exporter"`
	AmedasSite *configurationAmedasSite `json:"amedas_site"`
}

type configurationExporter struct {
	Addr string `json:"addr"`
}

type configurationAmedasSite struct {
	Sites []*site
}

type site struct {
	Pref string
	No   int
}

func (s *configurationAmedasSite) UnmarshalJSON(b []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	_, ok := data["site"]
	if !ok {
		return nil
	}
	multiSite, ok := data["site"].([]interface{})
	if ok {
		for _, m := range multiSite {
			for k, v := range m.(map[string]interface{}) {
				var sites []*site
				nos, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				if _, ok := nos["no"]; !ok {
					continue
				}
				n, ok := nos["no"].([]interface{})
				if ok {
					val := enumerable.Map(n, func(t interface{}) int { return int(t.(float64)) })
					for _, siteNo := range val {
						sites = append(sites, &site{Pref: k, No: siteNo})
					}
				}
				single, ok := nos["no"].(float64)
				if ok {
					sites = append(sites, &site{Pref: k, No: int(single)})
				}
				s.Sites = append(s.Sites, sites...)
			}
		}
	}
	return nil
}
