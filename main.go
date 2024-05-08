package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"github.com/theleeeo/file-butler/provider"
	"github.com/theleeeo/file-butler/server"
)

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

	providers, err := loadProviders()
	if err != nil {
		color.Red("ERROR: %s", err)
		return
	}

	srv, err := server.NewServer(server.Config{
		Addr:         viper.GetString("server.addr"),
		AllowRawBody: viper.GetBool("server.allow_raw_body"),
	})
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

	log.Println(srv.Run(context.Background()))
}

func loadProviders() ([]provider.Provider, error) {
	// Create a new viper instance for provider configs to not conflict with the main config
	pvp := viper.New()
	pvp.SetConfigName("providers")
	pvp.AddConfigPath(".")
	pvp.AddConfigPath("$HOME/.filebutler")
	pvp.AddConfigPath("/etc/filebutler/")
	err := pvp.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
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

	vMap, ok := v.(map[string]interface{})
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
