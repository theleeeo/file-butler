package authorization

import (
	"context"
	"fmt"

	authorization "github.com/theleeeo/file-butler/authorization/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewAllowTypesPlugin(args []string) (*AllowTypesPlugin, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("at least one request type is required")
	}

	allowedTypes := make([]authorization.RequestType, len(args))
	for i, arg := range args {
		var reqType authorization.RequestType
		switch arg {
		case "download":
			reqType = authorization.RequestType_REQUEST_TYPE_DOWNLOAD
		case "upload":
			reqType = authorization.RequestType_REQUEST_TYPE_UPLOAD
		case "get_tags", "get_metadata": // get_tags is here for backwards compatibility, can be removed once the /tags request is removed
			reqType = authorization.RequestType_REQUEST_TYPE_GET_METADATA
		case "list": // get_tags is here for backwards compatibility, can be removed once the /tags request is removed
			reqType = authorization.RequestType_REQUEST_TYPE_LIST
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown request type: "+arg)
		}

		allowedTypes[i] = authorization.RequestType(reqType)
	}

	return &AllowTypesPlugin{
		allowedRequestTypes: allowedTypes,
	}, nil
}

type AllowTypesPlugin struct {
	allowedRequestTypes []authorization.RequestType
}

func (p *AllowTypesPlugin) Authorize(ctx context.Context, req *authorization.AuthorizeRequest) error {
	for _, allowedType := range p.allowedRequestTypes {
		if req.RequestType == allowedType {
			return nil
		}
	}

	return status.Error(codes.PermissionDenied, "request type is not allowed")
}
