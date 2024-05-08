package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/theleeeo/file-butler/provider"
)

func NewServer(serverCfg Config, providerCfgs []provider.Config) (*Server, error) {
	s := &Server{
		cfg: serverCfg,
	}

	http.HandleFunc("/{provider}/{key}", s.handleUpload)
	http.HandleFunc("GET /{provider}/{key}", s.handleDownload)

	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           InternalErrorRedacter()(http.DefaultServeMux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}
	s.srv = srv

	s.providers = make(map[string]provider.Provider)

	for _, cfg := range providerCfgs {
		var p provider.Provider
		var err error

		switch cfg := cfg.(type) {
		case *provider.S3Config:
			p, err = provider.NewS3Provider(cfg)
		case *provider.NullConfig:
			p = &provider.NullProvider{}
		case *provider.LogConfig:
			p = &provider.LogProvider{}
		default:
			return nil, fmt.Errorf("unknown provider type: %T", cfg)
		}

		if err != nil {
			return nil, fmt.Errorf("unable to create provider: %w", err)
		}

		s.RegisterProvider(cfg.Id(), p)
	}

	return s, nil
}

type Config struct {
	Addr string

	// AllowRawBody allows to upload files using raw body data instead of multipart form data
	AllowRawBody bool
}

type Server struct {
	cfg Config

	mx        sync.RWMutex
	providers map[string]provider.Provider

	srv *http.Server
}

func (s *Server) RegisterProvider(id string, p provider.Provider) {
	log.Println("Registering provider", id)

	s.mx.Lock()
	defer s.mx.Unlock()

	s.providers[id] = p
}

func (s *Server) RemoveProvider(id string) {
	log.Println("Removing provider", id)

	s.mx.Lock()
	defer s.mx.Unlock()

	delete(s.providers, id)
}

func (s *Server) Run(ctx context.Context) error {
	log.Printf("Server is listening on %s", s.cfg.Addr)
	if err := s.srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" && r.Method != "PUT" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.PathValue("provider")

	s.mx.RLock()
	defer s.mx.RUnlock()

	p, ok := s.providers[providerName]
	if !ok {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	var data io.ReadCloser
	defer func() {
		if data != nil {
			data.Close()
		}
	}()

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		slog.Debug("Handling multipart form data", "Content-Length", r.ContentLength)

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			log.Println("Error parsing multipart form:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			log.Println("Error getting file from form:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data = file
	} else {
		if !s.cfg.AllowRawBody {
			http.Error(w, "raw body uploads are not allowed", http.StatusBadRequest)
			return
		}

		slog.Debug("Handling raw body data", "Content-Type", contentType, "Content-Length", r.ContentLength)
		data = r.Body
	}

	if err := p.PutObject(r.Context(), key, data); err != nil {
		if errors.Is(err, provider.ErrDenied) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")

	s.mx.RLock()
	defer s.mx.RUnlock()

	p, ok := s.providers[providerName]
	if !ok {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	data, err := p.GetObject(r.Context(), key)
	if err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if errors.Is(err, provider.ErrDenied) {
			http.Error(w, err.Error(), http.StatusForbidden)
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer data.Close()
	if _, err := io.Copy(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
