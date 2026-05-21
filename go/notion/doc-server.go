package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"go.f110.dev/notion-api/v3"
	"go.f110.dev/xerrors"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/k8s/volume"
	"go.f110.dev/mono/go/logger/slogger"
)

type databaseDocServerConfig struct {
	Id         string `yaml:"id"`
	DatabaseID string `yaml:"database_id"`
}

type DatabaseDocServer struct {
	configFile string
	token      string
	w          *volume.Watcher

	mu   sync.RWMutex
	conf []*databaseDocServerConfig

	s *http.Server
}

func NewDatabaseDocServer(addr, configPath, token string) (*DatabaseDocServer, error) {
	mux := http.NewServeMux()
	s := &DatabaseDocServer{
		configFile: configPath,
		token:      token,
		s: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
	mux.HandleFunc("/add", s.Add)
	s.loadConfig()

	if volume.CanWatchVolume(configPath) {
		mountPath, err := volume.FindMountPath(configPath)
		if err == nil {
			w, err := volume.NewWatcher(mountPath, s.loadConfig)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			s.w = w
		}
	}

	return s, nil
}

func (s *DatabaseDocServer) Add(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		b := struct {
			Id    string
			Title string
			Data  map[string]any
		}{}

		if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
			slogger.Log.Warn("Failed parse request body", slogger.E(err))
			return
		}
		slogger.Log.Info("Input data", slog.Any("body", b))

		var c *databaseDocServerConfig
		s.mu.RLock()
		for _, v := range s.conf {
			if v.Id == b.Id {
				c = v
				break
			}
		}
		s.mu.RUnlock()
		if c == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: s.token})
		tc := oauth2.NewClient(context.Background(), ts)
		client, err := notion.New(tc, notion.BaseURL)
		if err != nil {
			slogger.Log.Warn("Failed to create notion client", slogger.E(err), slog.String("id", c.Id))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		db, err := client.GetDatabase(req.Context(), c.DatabaseID)
		if err != nil {
			slogger.Log.Warn("Failed to get database", slogger.E(err), slog.String("id", c.Id), slog.String("database_id", c.DatabaseID))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		page, err := notion.NewPage(db, b.Title, nil)
		if err != nil {
			slogger.Log.Warn("Failed to initialize new page", slogger.E(err), slog.String("id", c.Id))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for k, v := range b.Data {
			prop, ok := db.Properties[k]
			if !ok {
				slogger.Log.Warn("Property not found", slog.String("key", k))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			switch prop.Type {
			case "number":
				num, ok := v.(float64)
				if !ok {
					slogger.Log.Info("The value is not float64", slog.Any("value", v), slog.Any("type", fmt.Sprintf("%T", v)))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				i := int(num)
				page.SetProperty(k, &notion.PropertyData{
					Type:   "number",
					Number: &i,
				})
			}
		}

		_, err = client.CreatePage(req.Context(), page)
		if err != nil {
			slogger.Log.Warn("Failed to create a page", slogger.E(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		slogger.Log.Info("Creating page was successfully")
	}
}

func (s *DatabaseDocServer) loadConfig() {
	f, err := os.Open(s.configFile)
	if err != nil {
		slogger.Log.Error("Failed to open config file", slogger.E(err), slog.String("path", s.configFile))
		return
	}
	var conf []*databaseDocServerConfig
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		slogger.Log.Error("Decode failure", slogger.E(err))
		return
	}
	s.mu.Lock()
	s.conf = conf
	s.mu.Unlock()
}

func (s *DatabaseDocServer) Start() error {
	slogger.Log.Info("Listen", slog.String("addr", s.s.Addr))
	return s.s.ListenAndServe()
}

func (s *DatabaseDocServer) Stop(ctx context.Context) error {
	slogger.Log.Info("Stopping server")
	if s.w != nil {
		s.w.Stop()
	}
	return s.s.Shutdown(ctx)
}
