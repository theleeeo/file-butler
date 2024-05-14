package authorization

import (
	"context"

	authorization "github.com/theleeeo/file-butler/authorization/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OnlyDownloadPlugin is a plugin that only allows download requests.
type OnlyDownloadPlugin struct{}

func (p *OnlyDownloadPlugin) Authorize(ctx context.Context, req *authorization.AuthorizeRequest) error {
	if req.RequestType == authorization.RequestType_REQUEST_TYPE_DOWNLOAD {
		return nil
	}

	return status.Error(codes.PermissionDenied, "only download requests are allowed")
}
