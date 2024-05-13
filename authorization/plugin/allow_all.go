package authorization

import (
	"context"
	"log"

	"github.com/theleeeo/file-butler/authorization/v1"
)

func NewAllowAllPlugin() *AllowAllPlugin {
	log.Println("Warning: Using the allow-all plugin, this is very unsafe and should only be used for testing purposes.")
	return &AllowAllPlugin{}
}

type AllowAllPlugin struct{}

func (p *AllowAllPlugin) Authorize(ctx context.Context, req *authorization.AuthorizeRequest) error {
	return nil
}
