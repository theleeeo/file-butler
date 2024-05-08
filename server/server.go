package server

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/theleeeo/file-butler/lerr"
	"github.com/theleeeo/file-butler/provider"
)

// NewServer creates a new server instance
// No providers are registered by default, they must be registered using the RegisterProvider method
func NewServer(serverCfg Config) (*Server, error) {
	if serverCfg.Addr == "" {
		return nil, errors.New("address is required")
	}

	s := &Server{
		allowRawBody: serverCfg.AllowRawBody,
		providers:    make(map[string]provider.Provider),
	}

	http.HandleFunc("/{provider}/{key}", s.handleUpload)
	http.HandleFunc("GET /{provider}/{key}", s.handleDownload)

	srv := &http.Server{
		Addr:              serverCfg.Addr,
		Handler:           InternalErrorRedacter()(http.DefaultServeMux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}
	s.srv = srv

	return s, nil
}

type Config struct {
	// Addr is the host:port the server will listen on
	Addr string

	// AllowRawBody allows to upload files using raw body data instead of multipart form data
	// Eg. This is what is used when uploading files using curl
	AllowRawBody bool
}

type Server struct {
	allowRawBody bool

	mx        sync.RWMutex
	providers map[string]provider.Provider

	srv *http.Server
}

func (s *Server) RegisterProvider(p provider.Provider) error {
	if p == nil || p.Id() == "" {
		return errors.New("provider is nil or has no ID")
	}

	id := p.Id()

	s.mx.RLock()
	if _, ok := s.providers[id]; ok {
		s.mx.RUnlock()
		return errors.New("provider already registered")
	}
	s.mx.RUnlock()

	log.Println("Registering provider", id)

	s.mx.Lock()
	defer s.mx.Unlock()

	s.providers[id] = p

	return nil
}

func (s *Server) RemoveProvider(id string) {
	log.Println("Removing provider", id)

	s.mx.Lock()
	defer s.mx.Unlock()

	delete(s.providers, id)
}

func (s *Server) ProviderIds() []string {
	s.mx.RLock()
	defer s.mx.RUnlock()

	var providerIds []string
	for _, p := range s.providers {
		providerIds = append(providerIds, p.Id())
	}

	return providerIds
}

func (s *Server) getProvider(id string) (provider.Provider, bool) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	p, ok := s.providers[id]
	return p, ok
}

func (s *Server) Run(ctx context.Context) error {
	log.Printf("Server is listening on %s", s.srv.Addr)
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
	p, ok := s.getProvider(providerName)
	if !ok {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	data, err := getDataSource(r, s.allowRawBody)
	if err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
		return
	}
	defer data.Close()

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

func getDataSource(r *http.Request, allowRawBody bool) (io.ReadCloser, error) {
	var data io.ReadCloser

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			log.Println("Error parsing multipart form:", err)

			return nil, lerr.Wrap("error parsing multipart form:", err, http.StatusBadRequest)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			log.Println("Error getting file from form:", err)

			return nil, lerr.Wrap("error getting file from form:", err, http.StatusBadRequest)
		}
		data = file
	} else {
		if !allowRawBody {
			log.Println("Attempted to upload raw body data when it is not allowed")

			return nil, lerr.New("raw body uploads are not allowed, use multipart form data", http.StatusBadRequest)
		}

		data = r.Body
	}

	return data, nil
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")

	p, ok := s.getProvider(providerName)
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
