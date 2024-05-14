package shared

import (
	"context"

	authorization "github.com/theleeeo/file-butler/authorization/v1"
)

type GRPCClient struct {
	Client authorization.AuthorizationServiceClient
}

func (g *GRPCClient) Authorize(ctx context.Context, req *authorization.AuthorizeRequest) error {
	_, err := g.Client.Authorize(ctx, req)
	return err
}
