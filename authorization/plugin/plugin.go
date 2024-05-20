package authorization

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/theleeeo/file-butler/authorization/shared"
	authorization "github.com/theleeeo/file-butler/authorization/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Plugin interface {
	Name() string
	Stop() error
	shared.Authorizer
}

// Config defines the configuration required to start a plugin.
//
// There are three types:
// - Cmd: An external binary or script that should be started using this command.
// - Addr: The host:port to an already running process that should be connected to. Only grpc is supported.
// - BuiltIn: If a built-in plugin should be used. These will not ran as a part of the file-butler application.
type Config struct {
	// If the plugin should be started by file-butler as another process:
	// Cmd is the command and args to run the plugin.
	// Example: "go run ./plugin/simple"
	Cmd []string
	// If the plugin is started as a standalone process:
	// Addr is the host:port the plugin listens on.
	Addr string
	// BuiltIn is the identifier for a built-in plugin.
	BuiltIn string
	// Args are the arguments to pass to the plugin. (only used with Cmd and BuiltIn)
	Args []string

	// Name is the unique name of the plugin used to identify it.
	Name string

	LogLevel hclog.Level
}

type pluginLogWriter struct {
	pluginName string
	io.Writer
}

func (w *pluginLogWriter) Write(p []byte) (n int, err error) {

	log.Printf("[plugin] %s: %s", w.pluginName, string(p))
	return len(p), nil
}

// NewPlugin creates a new plugin instance, either by starting a new process or connecting to an existing one.
// It is up to the caller to call Stop on the plugin when it is no longer needed.
// Check the documentation for the Config type for more information.
func NewPlugin(cfg Config) (Plugin, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("name must be set")
	}

	if cfg.LogLevel == hclog.NoLevel {
		cfg.LogLevel = hclog.Info
	}

	var pg Plugin

	if len(cfg.Cmd) != 0 {
		var args []string
		if len(cfg.Cmd) > 1 {
			args = cfg.Cmd[1:]
			args = append(args, cfg.Args...)
		}

		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig:  shared.Handshake,
			Plugins:          shared.PluginMap,
			Cmd:              exec.Command(cfg.Cmd[0], args...), //nolint:gosec // This is specified at startup. We trust a root server user.
			AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
			Logger:           hclog.New(&hclog.LoggerOptions{Level: cfg.LogLevel}),
			SyncStdout:       &pluginLogWriter{cfg.Name, os.Stdout},
			SyncStderr:       &pluginLogWriter{cfg.Name, os.Stderr},
		})

		rpcClient, err := client.Client()
		if err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}

		// Request the plugin
		raw, err := rpcClient.Dispense(shared.AuthPluginName)
		if err != nil {
			return nil, fmt.Errorf("failed to dispense plugin: %w", err)
		}

		auth, ok := raw.(shared.Authorizer)
		if !ok {
			return nil, fmt.Errorf("plugin does not implement Authorizer")
		}

		pg = &pluginClientWrapper{
			pluginBase: pluginBase{
				Authorizer: auth,
				name:       cfg.Name,
			},
			client: client,
		}
	}

	if cfg.Addr != "" {
		if pg != nil {
			return nil, fmt.Errorf("only one of Cmd, Addr or BuiltIn can be set")
		}

		conn, err := grpc.Dial(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("failed to dial: %w", err)
		}

		client := shared.GRPCClient{Client: authorization.NewAuthorizationServiceClient(conn)}

		pg = &grpcClientWrapper{
			pluginBase: pluginBase{
				Authorizer: &client,
				name:       cfg.Name,
			},
			conn: conn,
		}
	}

	if cfg.BuiltIn != "" {
		if pg != nil {
			return nil, fmt.Errorf("only one of Cmd, Addr or BuiltIn can be set")
		}

		switch cfg.BuiltIn {
		case "allow-types":
			a, err := NewAllowTypesPlugin(cfg.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to create allow-types plugin: %w", err)
			}

			pg = &pluginBase{
				Authorizer: a,
				name:       cfg.Name,
			}
		default:
			return nil, fmt.Errorf("unknown built-in plugin: %s", cfg.BuiltIn)
		}
	}

	if pg == nil {
		return nil, fmt.Errorf("must set either Cmd, Addr or Builtin")
	}

	return pg, nil
}

type pluginBase struct {
	shared.Authorizer
	name string
}

func (p *pluginBase) Name() string {
	return p.name
}

func (*pluginBase) Stop() error {
	return nil
}

type pluginClientWrapper struct {
	pluginBase
	client *plugin.Client
}

func (p *pluginClientWrapper) Stop() error {
	p.client.Kill()
	return nil
}

type grpcClientWrapper struct {
	pluginBase
	conn *grpc.ClientConn
}

func (g *grpcClientWrapper) Stop() error {
	return g.conn.Close()
}
