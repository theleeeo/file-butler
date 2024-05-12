package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	authPlugin "github.com/theleeeo/file-butler/authorization/plugin"
	"github.com/theleeeo/file-butler/authorization/v1"
	"github.com/theleeeo/file-butler/lerr"
	"github.com/theleeeo/file-butler/provider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Config struct {
	// Addr is the host:port the server will listen on
	Addr string

	// AllowRawBody allows to upload files using raw body data instead of multipart form data
	// Eg. This is what is used when uploading files using curl
	AllowRawBody bool

	// DefaultAuthPlugin is the name of the default auth plugin to use if the provider does not specify one
	DefaultAuthPlugin string
}

// NewServer creates a new server instance
// No providers are registered by default, they must be registered using the RegisterProvider method
func NewServer(serverCfg Config, plugins []authPlugin.Plugin) (*Server, error) {
	if serverCfg.Addr == "" {
		return nil, errors.New("address is required")
	}

	if serverCfg.DefaultAuthPlugin == "" {
		return nil, errors.New("default auth plugin is required")
	}

	if err := validateUniquePluginNames(plugins); err != nil {
		return nil, err
	}

	s := &Server{
		allowRawBody:      serverCfg.AllowRawBody,
		defaultAuthPlugin: serverCfg.DefaultAuthPlugin,
		providers:         make(map[string]provider.Provider),
		plugins:           plugins,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/{provider}/{key}", s.handleUpload)
	mux.HandleFunc("GET /{provider}/{key}", s.handleDownload)

	srv := &http.Server{
		Addr:              serverCfg.Addr,
		Handler:           InternalErrorRedacter()(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}
	s.srv = srv

	return s, nil
}

type Server struct {
	allowRawBody      bool
	defaultAuthPlugin string

	providerMx sync.RWMutex
	providers  map[string]provider.Provider
	// the plugins is a slice instead of a map because there will usually be a small number of plugins so it is not worth the overhead of a map
	plugins []authPlugin.Plugin

	srv *http.Server
}

func validateUniquePluginNames(providers []authPlugin.Plugin) error {
	pluginNames := make(map[string]struct{})
	for _, p := range providers {
		name := p.Name()
		if _, ok := pluginNames[name]; ok {
			return errors.New("plugin names must be unique, found duplicate: " + name)
		}
		pluginNames[name] = struct{}{}
	}

	return nil
}

func (s *Server) setPlugin(name string) authPlugin.Plugin {
	for _, p := range s.plugins {
		if p.Name() == name {
			return p
		}
	}

	return nil
}

func (s *Server) RegisterProvider(p provider.Provider) error {
	if p == nil || p.Id() == "" {
		return errors.New("provider is nil or has no ID")
	}

	id := p.Id()

	// If the provider specifies an auth plugin to use instead of the default one, make sure it exists
	if specifiedPlugin := p.AuthPlugin(); specifiedPlugin != "" {
		if plg := s.setPlugin(specifiedPlugin); plg == nil {
			return fmt.Errorf("auth plugin %s not found for provider %s", specifiedPlugin, id)
		}
	}

	s.providerMx.RLock()
	if _, ok := s.providers[id]; ok {
		s.providerMx.RUnlock()
		return errors.New("provider already registered")
	}
	s.providerMx.RUnlock()

	log.Println("Registering provider", id)

	s.providerMx.Lock()
	defer s.providerMx.Unlock()

	s.providers[id] = p

	return nil
}

func (s *Server) RemoveProvider(id string) {
	log.Println("Removing provider", id)

	s.providerMx.Lock()
	defer s.providerMx.Unlock()

	delete(s.providers, id)
}

func (s *Server) ProviderIds() []string {
	s.providerMx.RLock()
	defer s.providerMx.RUnlock()

	var providerIds []string
	for _, p := range s.providers {
		providerIds = append(providerIds, p.Id())
	}

	return providerIds
}

func (s *Server) getProvider(id string) provider.Provider {
	s.providerMx.RLock()
	defer s.providerMx.RUnlock()

	p, ok := s.providers[id]
	if !ok {
		return nil
	}
	return p
}

func (s *Server) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			log.Println("Error shutting down server:", err)
		}
	}()

	log.Printf("Server is listening on %s", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	return nil
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" && r.Method != "PUT" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r, key, p); err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
		return
	}

	dataSrc, err := getDataSource(r, s.allowRawBody)
	if err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
		return
	}
	defer dataSrc.Close()

	if err := p.PutObject(r.Context(), key, dataSrc); err != nil {
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

	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r, key, p); err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
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

func (s *Server) authorizeRequest(r *http.Request, key string, p provider.Provider) error {
	authPluginName := p.AuthPlugin()
	if authPluginName == "" {
		authPluginName = s.defaultAuthPlugin
	}

	authPlugin := s.setPlugin(authPluginName)
	if authPlugin == nil {
		return lerr.New("no auth plugin found for provider "+p.Id(), http.StatusInternalServerError)
	}

	var headers []*authorization.Header
	for k, v := range r.Header {
		headers = append(headers, &authorization.Header{
			Key:    k,
			Values: v,
		})
	}

	var reqType authorization.RequestType
	if r.Method == "GET" {
		reqType = authorization.RequestType_REQUEST_TYPE_DOWNLOAD
	} else if r.Method == "PUT" || r.Method == "POST" {
		reqType = authorization.RequestType_REQUEST_TYPE_UPLOAD
	} else {
		return lerr.New("unsupported request method", http.StatusMethodNotAllowed)
	}

	req := &authorization.AuthorizeRequest{
		Key:         key,
		Provider:    p.Id(),
		RequestType: reqType,
		Headers:     headers,
	}

	if err := authPlugin.Authorize(r.Context(), req); err != nil {
		s, ok := status.FromError(err)
		if !ok {
			return lerr.New(fmt.Sprintf("plugin error not a grpc status! error=%s", err.Error()), http.StatusInternalServerError)
		}

		if s.Code() == codes.Unauthenticated {
			return lerr.New(fmt.Sprintf("Unauthenticated: %s", s.Message()), http.StatusUnauthorized)
		}

		if s.Code() == codes.PermissionDenied {
			return lerr.New(fmt.Sprintf("permission denied: %s", s.Message()), http.StatusForbidden)
		}

		return lerr.New(fmt.Sprintf("plugin error: %s", s.Message()), http.StatusInternalServerError)
	}

	return nil
}
