package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	authorization "github.com/theleeeo/file-butler/authorization/v1"
	"github.com/theleeeo/file-butler/lerr"
	"github.com/theleeeo/file-butler/provider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	var reqType authorization.RequestType
	switch r.Method {
	case "GET":
		reqType = authorization.RequestType_REQUEST_TYPE_DOWNLOAD
	case "PUT", "POST":
		reqType = authorization.RequestType_REQUEST_TYPE_UPLOAD
	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}

	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/"+providerName+"/")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r.Context(), reqType, r.Header, key, p); err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
		return
	}

	if reqType == authorization.RequestType_REQUEST_TYPE_DOWNLOAD {
		data, err := s.handleDownload(r, p, key)
		if err != nil {
			http.Error(w, err.Error(), lerr.Code(err))
			return
		}

		defer data.Close()
		if _, err := io.Copy(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	if err := s.handleUpload(r, p, key); err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleUpload(r *http.Request, prov provider.Provider, key string) error {
	dataSrc, err := getDataSource(r, s.allowRawBody)
	if err != nil {
		return err
	}
	defer dataSrc.Close()

	if err := prov.PutObject(r.Context(), key, dataSrc); err != nil {
		if errors.Is(err, provider.ErrDenied) {
			return lerr.Wrap("error uploading object", err, http.StatusForbidden)
		}

		return lerr.New(err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func getDataSource(r *http.Request, allowRawBody bool) (io.ReadCloser, error) {
	var data io.ReadCloser

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			log.Println("error parsing multipart form:", err)

			return nil, lerr.Wrap("error parsing multipart form:", err, http.StatusBadRequest)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			log.Println("error getting file from form:", err)

			return nil, lerr.Wrap("error getting file from form:", err, http.StatusBadRequest)
		}
		data = file
	} else {
		if !allowRawBody {
			log.Println("attempted to upload raw body data when it is not allowed")

			return nil, lerr.New("raw body uploads are not allowed, use multipart form data", http.StatusUnsupportedMediaType)
		}

		data = r.Body
	}

	return data, nil
}

func (s *Server) handleDownload(r *http.Request, prov provider.Provider, key string) (io.ReadCloser, error) {
	data, err := prov.GetObject(r.Context(), key)
	if err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			return nil, lerr.New(err.Error(), http.StatusNotFound)
		}
		if errors.Is(err, provider.ErrDenied) {
			return nil, lerr.New(err.Error(), http.StatusForbidden)
		}

		return nil, lerr.New(err.Error(), http.StatusInternalServerError)
	}

	return data, nil
}

func (s *Server) authorizeRequest(ctx context.Context, reqType authorization.RequestType, headers map[string][]string, key string, p provider.Provider) error {
	authPluginName := p.AuthPlugin()
	if authPluginName == "" {
		authPluginName = s.defaultAuthPlugin
	}

	authPlugin := s.setPlugin(authPluginName)
	if authPlugin == nil {
		return lerr.New("no auth plugin found for provider "+p.Id(), http.StatusInternalServerError)
	}

	var authHeaderMap []*authorization.Header
	for k, v := range headers {
		authHeaderMap = append(authHeaderMap, &authorization.Header{
			Key:    k,
			Values: v,
		})
	}

	req := &authorization.AuthorizeRequest{
		Key:         key,
		Provider:    p.Id(),
		RequestType: reqType,
		Headers:     authHeaderMap,
	}

	if err := authPlugin.Authorize(ctx, req); err != nil {
		s, ok := status.FromError(err)
		if !ok {
			return lerr.New(fmt.Sprintf("plugin error not a grpc status! error=%s", err.Error()), http.StatusInternalServerError)
		}

		if s.Code() == codes.Unauthenticated {
			return lerr.New(fmt.Sprintf("Unauthenticated: %s", s.Message()), http.StatusUnauthorized)
		}

		if s.Code() == codes.PermissionDenied {
			return lerr.New(fmt.Sprintf("permission denied: %s", s.Message()), http.StatusForbidden)
		}

		return lerr.New(fmt.Sprintf("plugin error: %s", s.Message()), http.StatusInternalServerError)
	}

	return nil
}

func (s *Server) handlePresign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	presigner, ok := p.(provider.Presigner)
	if !ok {
		http.Error(w, provider.ErrNoPresign.Error(), http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/presign/"+providerName+"/")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	op := r.URL.Query().Get("op")
	if op == "" {
		http.Error(w, "presign operation is required", http.StatusBadRequest)
		return
	}

	var presignOp provider.PresignOperation
	var reqType authorization.RequestType
	switch op {
	case "download":
		presignOp = provider.PresignOperationDownload
		reqType = authorization.RequestType_REQUEST_TYPE_DOWNLOAD
	case "upload":
		presignOp = provider.PresignOperationUpload
		reqType = authorization.RequestType_REQUEST_TYPE_UPLOAD
	default:
		http.Error(w, fmt.Sprint("unsupported presign operation: ", op), http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r.Context(), reqType, r.Header, key, p); err != nil {
		http.Error(w, err.Error(), lerr.Code(err))
		return
	}

	url, err := presigner.PresignURL(r.Context(), key, presignOp)
	if err != nil {
		if errors.Is(err, provider.ErrDenied) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		if errors.Is(err, provider.ErrNoPresign) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write([]byte(url))
}
