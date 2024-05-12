package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	authPlugin "github.com/theleeeo/file-butler/authorization/plugin"
	"github.com/theleeeo/file-butler/provider"
	"github.com/theleeeo/file-butler/server"
)

func loadPlugins() ([]authPlugin.Plugin, error) {
	var cfgs []authPlugin.Config
	err := viper.UnmarshalKey("auth-plugins", &cfgs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configs: %w", err)
	}

	var plugins []authPlugin.Plugin

	for _, cfg := range cfgs {
		pg, err := authPlugin.NewPlugin(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create plugin %s: %w", cfg.Name, err)
		}
		plugins = append(plugins, pg)
	}

	return plugins, nil
}

func main() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.filebutler")
	viper.AddConfigPath("/etc/filebutler/")
	err := viper.ReadInConfig()
	if err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if errors.As(err, &notFoundErr) {
			color.Red("config file not found")
			return
		}

		color.Red("error reading config file: %v", err)
		return
	}

	// Create a new viper instance for provider configs to not conflict with the main config
	pvp := viper.New()
	providers, err := loadProviders(pvp)
	if err != nil {
		color.Red("ERROR: %s", err)
		return
	}

	plugins, err := loadPlugins()
	if err != nil {
		color.Red("ERROR: %s", err)
		return
	}

	srv, err := server.NewServer(server.Config{
		Addr:              viper.GetString("server.addr"),
		AllowRawBody:      viper.GetBool("server.allow_raw_body"),
		DefaultAuthPlugin: viper.GetString("server.default_auth_plugin"),
	}, plugins)
	if err != nil {
		color.Red("ERROR creating server: %s", err)
		return
	}

	for _, p := range providers {
		if err := srv.RegisterProvider(p); err != nil {
			color.Red("ERROR registering provider %s: %s", p.Id(), err)
			return
		}
	}

	pvp.WatchConfig()
	pvp.OnConfigChange(reloadProvidersFunc(pvp, srv))

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		<-signalChan
		log.Println("Stopping server")
		cancel()

		// If another signal is received, force kill the server
		<-signalChan
		log.Println("Force killing")
		os.Exit(1)
	}()

	if err := srv.Run(ctx); err != nil {
		color.Red("ERROR running server: %s", err)
	}

	log.Println("Server stopped")

	for _, p := range plugins {
		if err := p.Stop(); err != nil {
			color.Red("ERROR stopping plugin %s: %s", p.Name(), err)
		}
	}
}

func reloadProvidersFunc(pvp *viper.Viper, srv *server.Server) func(fsnotify.Event) {
	return func(in fsnotify.Event) {
		log.Println("Reloading providers")

		newProviders, err := loadProviders(pvp)
		if err != nil {
			log.Println(color.RedString("ERROR: %s", err))
			return
		}

		// All the providers that are in the new config shuld be registered
		// They are removed first in case they existed before and should be reloaded
		for _, p := range newProviders {
			srv.RemoveProvider(p.Id())

			if err := srv.RegisterProvider(p); err != nil {
				log.Println(color.RedString("ERROR registering provider %s: %s", p.Id(), err))
			}
		}

		// All the providers that are not in the new config but existed before should be removed
		var providersToDelete []string
		for _, p := range srv.ProviderIds() {
			if !providersContains(newProviders, p) {
				providersToDelete = append(providersToDelete, p)
			}
		}

		for _, id := range providersToDelete {
			srv.RemoveProvider(id)
		}
	}
}

func providersContains(providers []provider.Provider, id string) bool {
	for _, p := range providers {
		if p.Id() == id {
			return true
		}
	}
	return false

}

// loadProviders loads the provider configurations from the provider config file
func loadProviders(pvp *viper.Viper) ([]provider.Provider, error) {
	pvp.SetConfigName("providers")
	pvp.AddConfigPath(".")
	pvp.AddConfigPath("$HOME/.filebutler")
	pvp.AddConfigPath("/etc/filebutler/")
	err := pvp.ReadInConfig()
	if err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("provider file not found")
		}

		return nil, fmt.Errorf("error reading provider file: %w", err)
	}

	var providers []provider.Provider
	for id, v := range pvp.AllSettings() {
		providerType := pvp.GetString(fmt.Sprintf("%s.type", id))
		if providerType == "" {
			return nil, fmt.Errorf("type is required, missing for: %s", id)
		}

		var cfg provider.Config
		var err error

		// Parse the provider config based on the provider type
		switch providerType {
		case string(provider.ProviderTypeVoid):
			cfg, err = unmarshalProviderCfg[*provider.VoidConfig](id, v)
		case string(provider.ProviderTypeLog):
			cfg, err = unmarshalProviderCfg[*provider.LogConfig](id, v)
		case string(provider.ProviderTypeS3):
			cfg, err = unmarshalProviderCfg[*provider.S3Config](id, v)
		default:
			return nil, fmt.Errorf("unknown provider type: %s", providerType)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to parse config of provider %s: %w", id, err)
		}

		// Create the provider based on the config
		// It is kept separate from the above part because IMO it is cleaner
		var p provider.Provider

		switch cfg := cfg.(type) {
		case *provider.S3Config:
			p, err = provider.NewS3Provider(cfg)
		case *provider.VoidConfig:
			p = provider.NewVoidProvider(cfg)
		case *provider.LogConfig:
			p = provider.NewLogProvider(cfg)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to create provider %s: %w", id, err)
		}

		providers = append(providers, p)
	}

	return providers, nil
}

func unmarshalProviderCfg[T provider.Config](id string, v any) (T, error) {
	var cfg T

	vMap, ok := v.(map[string]any)
	if !ok {
		return cfg, fmt.Errorf("invalid config type: %T", v)
	}

	vMap["id"] = id

	jsonBytes, err := json.Marshal(vMap)
	if err != nil {
		return cfg, fmt.Errorf("unable to marshal config: %w", err)
	}

	err = json.Unmarshal(jsonBytes, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	return cfg, nil
}
