package shared

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/go-plugin"
	"github.com/theleeeo/file-butler/authorization/v1"
)

const (
	AuthPluginName = "auth"
)

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "FileButlerPluginKey",
	MagicCookieValue: "ILovePenguins",
}

var PluginMap = map[string]plugin.Plugin{
	AuthPluginName: &AuthPlugin{},
}

type Authorizer interface {
	Authorize(context.Context, *authorization.AuthorizeRequest) error
}

type AuthPlugin struct {
	plugin.Plugin
	Impl Authorizer
}

func (p *AuthPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	authorization.RegisterAuthorizationServiceServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *AuthPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{Client: authorization.NewAuthorizationServiceClient(c)}, nil
}
