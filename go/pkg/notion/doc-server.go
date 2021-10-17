package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"go.f110.dev/notion-api/v2"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"

	"go.f110.dev/mono/go/pkg/logger"
)

type config struct {
	Id         string `yaml:"id"`
	Token      string `yaml:"token"`
	DatabaseID string `yaml:"database_id"`
}

type DatabaseDocServer struct {
	conf []*config
	s    *http.Server
}

func NewDatabaseDocServer(addr, configPath string) (*DatabaseDocServer, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	var conf []*config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	mux := http.NewServeMux()
	s := &DatabaseDocServer{
		conf: conf,
		s: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
	mux.HandleFunc("/add", s.Add)

	return s, nil
}

func (s *DatabaseDocServer) Add(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		b := struct {
			Id    string
			Value string
		}{}

		if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
			logger.Log.Warn("Failed parse request body", zap.Error(err))
			return
		}

		var c *config
		for _, v := range s.conf {
			if v.Id == b.Id {
				c = v
				break
			}
		}
		if c == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.Token})
		tc := oauth2.NewClient(context.Background(), ts)
		client, err := notion.New(tc, notion.BaseURL)
		if err != nil {
			logger.Log.Warn("Failed to create notion client", zap.Error(err), zap.String("id", c.Id))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		db, err := client.GetDatabase(req.Context(), c.DatabaseID)
		if err != nil {
			logger.Log.Warn("Failed to get database", zap.Error(err), zap.String("id", c.Id), zap.String("database_id", c.DatabaseID))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		page, err := notion.NewPage(db, b.Value, nil)
		if err != nil {
			logger.Log.Warn("Failed to initialize new page", zap.Error(err), zap.String("id", c.Id))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, err = client.CreatePage(req.Context(), page)
		if err != nil {
			logger.Log.Warn("Failed to create a page", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		logger.Log.Info("Creating page was successfully")
	}
}

func (s *DatabaseDocServer) Start() error {
	logger.Log.Info("Listen", zap.String("addr", s.s.Addr))
	return s.s.ListenAndServe()
}

func (s *DatabaseDocServer) Stop(ctx context.Context) error {
	logger.Log.Info("Stopping server")
	return s.s.Shutdown(ctx)
}
