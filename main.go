package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"github.com/theleeeo/file-butler/provider"
	"github.com/theleeeo/file-butler/server"
)

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
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

	providerCfgs, err := loadProviders()
	if err != nil {
		color.Red("ERROR: %s", err)
		return
	}

	for _, cfg := range providerCfgs {
		fmt.Printf("provider: %s\n", cfg.Id())
	}

	srv, err := server.NewServer(server.Config{
		Addr:         viper.GetString("server.addr"),
		AllowRawBody: viper.GetBool("server.allow_raw_body"),
	}, providerCfgs)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(srv.Run(context.Background()))
}

func loadProviders() ([]provider.Config, error) {
	// Create a new viper instance for provider configs to not conflict with the main config
	pvp := viper.New()
	pvp.SetConfigName("providers")
	pvp.SetConfigType("toml")
	pvp.AddConfigPath(".")
	pvp.AddConfigPath("$HOME/.filebutler")
	pvp.AddConfigPath("/etc/filebutler/")
	err := pvp.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found")
		}

		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var providerConfigs []provider.Config
	for id, v := range pvp.AllSettings() {
		providerType := pvp.GetString(fmt.Sprintf("%s.type", id))
		if providerType == "" {
			return nil, fmt.Errorf("provider type is required, missing for: %s", id)
		}

		var cfg provider.Config
		var err error

		switch providerType {
		case string(provider.ProviderTypeNull):
			cfg, err = unmarshalProviderCfg[*provider.NullConfig](id, v)
		case string(provider.ProviderTypeLog):
			cfg, err = unmarshalProviderCfg[*provider.LogConfig](id, v)
		case string(provider.ProviderTypeS3):
			cfg, err = unmarshalProviderCfg[*provider.S3Config](id, v)
		default:
			return nil, fmt.Errorf("unknown provider type: %s", providerType)
		}

		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal provider config: %w", err)
		}

		providerConfigs = append(providerConfigs, cfg)
	}

	return providerConfigs, nil
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
