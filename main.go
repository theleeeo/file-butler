package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			color.Red("config file not found")
			return
		}

		color.Red("error reading config file: %v", err)
		return
	}

	// Create a new viper instance for provider configs to not conflict with the main config
	pvp := viper.New()
	providers, _, err := loadProviders(pvp, nil)
	if err != nil {
		color.Red("ERROR: %s", err)
		return
	}

	plugins, err := loadPlugins()
	if err != nil {
		color.Red("ERROR: %s", err)
		return
	}
	for _, p := range plugins {
		defer p.Stop()
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

	log.Println(srv.Run(context.Background()))
}

func reloadProvidersFunc(pvp *viper.Viper, srv *server.Server) func(fsnotify.Event) {
	return func(in fsnotify.Event) {
		log.Println("Reloading providers")

		currentProviders := srv.ProviderIds()

		newProviders, removedProviders, err := loadProviders(pvp, currentProviders)
		if err != nil {
			log.Println(color.RedString("ERROR: %s", err))
			return
		}

		if len(newProviders) == 0 && len(removedProviders) == 0 {
			log.Println("No changes to the providers detected")
			return
		}

		for _, p := range newProviders {
			if err := srv.RegisterProvider(p); err != nil {
				log.Println(color.RedString("ERROR registering provider %s: %s", p.Id(), err))
			}
		}

		for _, id := range removedProviders {
			srv.RemoveProvider(id)
		}
	}
}

// loadProviders loads the provider configurations from the provider config file
// It takes an optional slice of the IDs of the currently registered providers so that it will not duplicate the providers during a live reload.
//
// It returns a slice of instantiated providers and a slice of the IDs of the providers that should be removed since they does not exist in the new config.
func loadProviders(pvp *viper.Viper, registeredProviders []string) ([]provider.Provider, []string, error) {
	pvp.SetConfigName("providers")
	pvp.AddConfigPath(".")
	pvp.AddConfigPath("$HOME/.filebutler")
	pvp.AddConfigPath("/etc/filebutler/")
	err := pvp.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, nil, fmt.Errorf("provider file not found")
		}

		return nil, nil, fmt.Errorf("error reading provider file: %w", err)
	}

	var providers []provider.Provider
ProviderLoop:
	for id, v := range pvp.AllSettings() {
		// Check if the provider is already registered
		for i, currentId := range registeredProviders {
			if id == currentId {
				// Remove the provider from the list of current providers so that it will not be removed later
				registeredProviders = append(registeredProviders[:i], registeredProviders[i+1:]...)
				continue ProviderLoop
			}
		}

		providerType := pvp.GetString(fmt.Sprintf("%s.type", id))
		if providerType == "" {
			return nil, nil, fmt.Errorf("type is required, missing for: %s", id)
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
			return nil, nil, fmt.Errorf("unknown provider type: %s", providerType)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("unable to parse config of provider %s: %w", id, err)
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
			return nil, nil, fmt.Errorf("unable to create provider %s: %w", id, err)
		}

		providers = append(providers, p)
	}

	// The currentProviders slice now contains the IDs of the already registered providers that are not in the new config
	return providers, registeredProviders, nil
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
