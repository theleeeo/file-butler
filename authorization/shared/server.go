package shared

import (
	"context"

	authorization "github.com/theleeeo/file-butler/authorization/v1"
)

type GRPCServer struct {
	Impl Authorizer
}

func (g *GRPCServer) Authorize(ctx context.Context, req *authorization.AuthorizeRequest) (*authorization.AuthorizeResponse, error) {
	err := g.Impl.Authorize(ctx, req)
	return &authorization.AuthorizeResponse{}, err
}
