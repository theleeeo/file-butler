package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	authPlugin "github.com/theleeeo/file-butler/authorization/plugin"
	"github.com/theleeeo/file-butler/provider"
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

func defaultAuthPluginExists(plugins []authPlugin.Plugin, defaultAuthPlugin string) bool {
	for _, p := range plugins {
		if p.Name() == defaultAuthPlugin {
			return true
		}
	}

	return false
}

// NewServer creates a new server instance
// No providers are registered by default, they must be registered using the RegisterProvider method
func NewServer(serverCfg Config, plugins []authPlugin.Plugin) (*Server, error) {
	if serverCfg.Addr == "" {
		return nil, errors.New("address is required")
	}

	if len(plugins) == 0 {
		return nil, errors.New("at least one auth plugin is required")
	}

	if serverCfg.DefaultAuthPlugin == "" {
		return nil, errors.New("default auth plugin is required")
	}

	if !defaultAuthPluginExists(plugins, serverCfg.DefaultAuthPlugin) {
		return nil, errors.New("default auth plugin not found")
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
	mux.HandleFunc("/{provider}/", s.handleFile)

	mux.HandleFunc("GET /presign/{provider}/{key}", s.handlePresign)

	s.srv = &http.Server{
		Addr:              serverCfg.Addr,
		Handler:           InternalErrorRedacter()(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

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
