package server

import (
	"context"
	"encoding/json"
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

const (
	allowUnknownContentLength = true
)

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	var reqType authorization.RequestType
	switch r.Method {
	case http.MethodGet:
		reqType = authorization.RequestType_REQUEST_TYPE_DOWNLOAD
	case http.MethodPut, http.MethodPost:
		reqType = authorization.RequestType_REQUEST_TYPE_UPLOAD
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/file/"+providerName+"/")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r.Context(), reqType, r.Header, key, p); err != nil {
		lerr.ToHTTP(w, err)
		return
	}

	if reqType == authorization.RequestType_REQUEST_TYPE_DOWNLOAD {
		data, objectInfo, err := s.handleDownload(r, p, key)
		if err != nil {
			if errors.Is(err, provider.ErrNotModified) {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			lerr.ToHTTP(w, err)
			return
		}
		defer data.Close()

		if objectInfo.LastModified != nil {
			w.Header().Set("Last-Modified", objectInfo.LastModified.Format(http.TimeFormat))
		}

		if objectInfo.ContentLength != nil {
			w.Header().Set("Content-Length", fmt.Sprint(*objectInfo.ContentLength))
		}

		if objectInfo.ContentType != nil {
			w.Header().Set("Content-Type", *objectInfo.ContentType)
		}

		if _, err := io.Copy(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	if err := s.handleUpload(r, p, key); err != nil {
		lerr.ToHTTP(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleUpload(r *http.Request, prov provider.Provider, key string) error {
	dataSrc, err := getDataSource(r, s.allowRawBody)
	if err != nil {
		return err
	}
	defer dataSrc.Close()

	contentLength := r.ContentLength
	if contentLength == 0 {
		return lerr.New(http.StatusBadRequest, "no content to upload")
	}

	// The content length is unknown
	if contentLength < 0 {
		if !allowUnknownContentLength {
			return lerr.New(http.StatusBadRequest, "content length must be set")
		}

		body, err := io.ReadAll(dataSrc)
		if err != nil {
			fmt.Println("error:", err)
			return err
		}
		contentLength = int64(len(body))
		dataSrc = io.NopCloser(strings.NewReader(string(body)))
	}

	tags, err := parseTags(r.URL.Query()["tag"])
	if err != nil {
		return err
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(nil)
	}

	if err := prov.PutObject(r.Context(), key, dataSrc, provider.PutOptions{
		ContentType:   contentType,
		ContentLength: contentLength,
		Tags:          tags,
	}); err != nil {
		if errors.Is(err, provider.ErrDenied) {
			return lerr.Wrap(err, http.StatusForbidden, "error uploading object")
		}

		return lerr.New(http.StatusInternalServerError, err.Error())
	}

	return nil
}

func parseTags(rawTags []string) (map[string]string, error) {
	if len(rawTags) == 0 {
		return nil, nil
	}

	parsedTags := make(map[string]string)
	for _, tag := range rawTags {
		parts := strings.Split(tag, ":")
		if len(parts) != 2 {
			return nil, lerr.New(http.StatusBadRequest, "invalid tag format")
		}

		if parts[0] == "" || parts[1] == "" {
			return nil, lerr.New(http.StatusBadRequest, "invalid tag format")
		}

		if _, ok := parsedTags[parts[0]]; ok {
			return nil, lerr.Newf(http.StatusBadRequest, "multiple values for key %s, this is not supported", parts[0])
		}

		parsedTags[parts[0]] = parts[1]
	}

	return parsedTags, nil
}

func getDataSource(r *http.Request, allowRawBody bool) (io.ReadCloser, error) {
	var data io.ReadCloser

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			log.Println("error parsing multipart form:", err)

			return nil, lerr.Wrap(err, http.StatusBadRequest, "error parsing multipart form:")
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			log.Println("error getting file from form:", err)

			return nil, lerr.Wrap(err, http.StatusBadRequest, "error getting file from form:")
		}
		data = file
	} else {
		if !allowRawBody {
			log.Println("attempted to upload raw body data when it is not allowed")

			return nil, lerr.New(http.StatusUnsupportedMediaType, "raw body uploads are not allowed, use multipart form data")
		}

		data = r.Body
	}

	return data, nil
}

func (s *Server) handleDownload(r *http.Request, prov provider.Provider, key string) (io.ReadCloser, provider.ObjectInfo, error) {
	opts := provider.GetOptions{}

	if r.Header.Get("If-Modified-Since") != "" {
		t, err := http.ParseTime(r.Header.Get("If-Modified-Since"))
		if err != nil {
			return nil, provider.ObjectInfo{}, lerr.New(http.StatusBadRequest, "invalid If-Modified-Since header")
		}

		opts.LastModified = &t
	}

	data, objectInfo, err := prov.GetObject(r.Context(), key, opts)
	if err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			return nil, provider.ObjectInfo{}, lerr.Newf(http.StatusNotFound, err.Error())
		}
		if errors.Is(err, provider.ErrDenied) {
			return nil, provider.ObjectInfo{}, lerr.New(http.StatusForbidden, err.Error())
		}
		if errors.Is(err, provider.ErrNotModified) {
			return nil, provider.ObjectInfo{}, err
		}

		return nil, provider.ObjectInfo{}, lerr.Newf(http.StatusInternalServerError, "error getting object from provider: %s", err.Error())
	}

	return data, objectInfo, nil
}

func (s *Server) authorizeRequest(ctx context.Context, reqType authorization.RequestType, headers map[string][]string, key string, p provider.Provider) error {
	authPluginName := p.AuthPlugin()
	if authPluginName == "" {
		authPluginName = s.defaultAuthPlugin
	}

	authPlugin := s.setPlugin(authPluginName)
	if authPlugin == nil {
		return lerr.Newf(http.StatusInternalServerError, "no auth plugin found for provider %s", p.Id())
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
			return lerr.Newf(http.StatusInternalServerError, "plugin error not a grpc status! error=%s", err.Error())
		}

		if s.Code() == codes.Unauthenticated {
			return lerr.Newf(http.StatusUnauthorized, "Unauthenticated: %s", s.Message())
		}

		if s.Code() == codes.PermissionDenied {
			return lerr.Newf(http.StatusForbidden, "permission denied: %s", s.Message())
		}

		return lerr.Newf(http.StatusInternalServerError, "plugin error: %s", s.Message())
	}

	return nil
}

func (s *Server) handlePresign(w http.ResponseWriter, r *http.Request) {
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
		lerr.ToHTTP(w, err)
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

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/meta/"+providerName+"/")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r.Context(), authorization.RequestType_REQUEST_TYPE_GET_METADATA, r.Header, key, p); err != nil {
		lerr.ToHTTP(w, err)
		return
	}

	tags, err := p.GetTags(r.Context(), key)
	if err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type metadataResponse struct {
		Tags map[string]string `json:"tags,omitempty"`
	}

	metadata := metadataResponse{
		Tags: tags,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		log.Println("error encoding metadata:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/tags/"+providerName+"/")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRequest(r.Context(), authorization.RequestType_REQUEST_TYPE_GET_METADATA, r.Header, key, p); err != nil {
		lerr.ToHTTP(w, err)
		return
	}

	tags, err := p.GetTags(r.Context(), key)
	if err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tags); err != nil {
		log.Println("error encoding tags:", err)
		// If some part of the response was able to be written, the client will already have received a partial response and status code 200.
		// This is for if the response was not able to be written at all.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	p := s.getProvider(providerName)
	if p == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	prefix := strings.TrimPrefix(r.URL.Path, "/list/"+providerName+"/")
	fmt.Println("prefix:", prefix)

	if err := s.authorizeRequest(r.Context(), authorization.RequestType_REQUEST_TYPE_LIST, r.Header, prefix, p); err != nil {
		lerr.ToHTTP(w, err)
		return
	}

	objects, err := p.ListObjects(r.Context(), prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(objects); err != nil {
		log.Println("error encoding list:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
