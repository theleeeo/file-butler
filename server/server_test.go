package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	authPlugin "github.com/theleeeo/file-butler/authorization/plugin"
	"github.com/theleeeo/file-butler/mocks"
	"github.com/theleeeo/file-butler/provider"
)

func getValidPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func ptrTo[T any](v T) *T {
	return &v
}

func Test_CreateServer(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"upload"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	t.Run("Register provider", func(t *testing.T) {
		err := srv.RegisterProvider(provider.NewVoidProvider(&provider.VoidConfig{
			ConfigBase: provider.ConfigBase{
				ID: "void",
			},
		}))
		assert.NoError(t, err)

		assert.Equal(t, []string{"void"}, srv.ProviderIds())
	})

	t.Run("Remove provider", func(t *testing.T) {
		srv.RemoveProvider("void")
		assert.Empty(t, srv.ProviderIds())
	})
}

func Test_Download_NotAllowed(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"upload"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Get object", func(t *testing.T) {
		prov.On("GetObject", mock.Anything, "123", provider.GetOptions{}).Return(
			"hello", provider.ObjectInfo{}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/file/mock/123", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "permission denied: request type is not allowed\n", string(d))
	})
}

func Test_Download(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"download"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Get object", func(t *testing.T) {
		prov.On("GetObject", mock.Anything, "123", provider.GetOptions{}).Return(
			"hello", provider.ObjectInfo{}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/file/mock/123", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(d))
	})

	t.Run("Multi-slash key", func(t *testing.T) {
		prov.On("GetObject", mock.Anything, "123/456/abc", provider.GetOptions{}).Return(
			"hello", provider.ObjectInfo{}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/file/mock/123/456/abc", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(d))
	})
}

func Test_Upload_NotAllowed(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"download"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
		AllowRawBody:      true,
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Put object", func(t *testing.T) {
		prov.On("PutObject", mock.Anything, "123", []byte("hello"), int64(5), map[string]string(nil)).Return(nil).Once()

		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/file/mock/123", port), strings.NewReader("hello"))
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "permission denied: request type is not allowed\n", string(d))
	})
}
func Test_Upload(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"upload"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
		AllowRawBody:      true,
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Put object", func(t *testing.T) {
		prov.On("PutObject", mock.Anything, "123", []byte("hello"), int64(5), map[string]string(nil)).Return(nil).Once()

		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/file/mock/123", port), strings.NewReader("hello"))
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Multi-slash key", func(t *testing.T) {
		prov.On("PutObject", mock.Anything, "123/456/abc", []byte("hello"), int64(5), map[string]string(nil)).Return(nil).Once()

		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/file/mock/123/456/abc", port), strings.NewReader("hello"))
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("With tags", func(t *testing.T) {
		prov.On("PutObject", mock.Anything, "123/456/abc", []byte("hello"), int64(5), map[string]string{
			"abc":  "123",
			"pepe": "frog",
		}).Return(nil).Once()

		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/file/mock/123/456/abc?tag=abc:123&tag=pepe:frog", port), strings.NewReader("hello"))
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Double of same tag not allowed", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/file/mock/123/456/abc?tag=abc:123&tag=abc:123", port), strings.NewReader("hello"))
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "multiple values for key abc, this is not supported\n", string(d))
	})
}

func Test_Upload_DontAllowRawBody(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"upload"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
		AllowRawBody:      false,
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Put object", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/file/mock/123", port), strings.NewReader("hello"))
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)

		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "raw body uploads are not allowed, use multipart form data\n", string(d))
	})
}

func Test_Presign(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"download", "upload"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewPresignProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("download", func(t *testing.T) {
		prov.On("PresignURL", mock.Anything, "123", provider.PresignOperationDownload).Return(
			"presignHello", nil).Once()

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/presign/mock/123?op=download", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "presignHello", string(d))
	})

	t.Run("download, multi-slash key", func(t *testing.T) {
		prov.On("PresignURL", mock.Anything, "123/456/abc", provider.PresignOperationDownload).Return(
			"presignHello", nil).Once()

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/presign/mock/123/456/abc?op=download", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "presignHello", string(d))
	})

	t.Run("upload", func(t *testing.T) {
		prov.On("PresignURL", mock.Anything, "123", provider.PresignOperationUpload).Return(
			"presignHello", nil).Once()

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/presign/mock/123?op=upload", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "presignHello", string(d))
	})
}

func Test_GetTags(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"get_tags"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Get tags", func(t *testing.T) {
		prov.On("GetTags", mock.Anything, "123").Return(
			map[string]string{"hello": "world"}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/tags/mock/123", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"hello":"world"}`, string(d))
	})

	t.Run("Multi-slash key", func(t *testing.T) {
		prov.On("GetTags", mock.Anything, "123/456/abc").Return(
			map[string]string{"hello": "world"}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/tags/mock/123/456/abc", port), nil)
		assert.NoError(t, err)

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"hello":"world"}`, string(d))
	})
}

func Test_Download_WithLastModified(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-types",
		Args:    []string{"download"},
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := NewServer(Config{
		Addr:              fmt.Sprint("localhost:", port),
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)

	prov := mocks.NewProvider(provider.ConfigBase{ID: "mock"})
	err = srv.RegisterProvider(prov)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Run("Object not modified", func(t *testing.T) {
		prov.On("GetObject", mock.Anything, "123", provider.GetOptions{LastModified: ptrTo(time.Unix(100, 0).UTC())}).Return(
			nil, provider.ObjectInfo{}, provider.ErrNotModified).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/file/mock/123", port), nil)
		assert.NoError(t, err)
		req.Header.Set("If-Modified-Since", time.Unix(100, 0).UTC().Format(http.TimeFormat))

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "", string(d))
	})

	t.Run("Multi-slash key", func(t *testing.T) {
		prov.On("GetObject", mock.Anything, "123/456/abc", provider.GetOptions{LastModified: ptrTo(time.Unix(100, 0).UTC())}).Return(
			nil, provider.ObjectInfo{}, provider.ErrNotModified).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/file/mock/123/456/abc", port), nil)
		assert.NoError(t, err)
		req.Header.Set("If-Modified-Since", time.Unix(100, 0).UTC().Format(http.TimeFormat))

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "", string(d))
	})

	t.Run("Object is modified", func(t *testing.T) {
		prov.On("GetObject", mock.Anything, "123", provider.GetOptions{LastModified: ptrTo(time.Unix(100, 0).UTC())}).Return(
			"hello", provider.ObjectInfo{LastModified: ptrTo(time.Unix(100, 0).UTC())}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/file/mock/123", port), nil)
		assert.NoError(t, err)
		req.Header.Set("If-Modified-Since", time.Unix(100, 0).UTC().Format(http.TimeFormat))

		client := http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		d, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(d))

		assert.Equal(t, time.Unix(100, 0).UTC().Format(http.TimeFormat), resp.Header.Get("Last-Modified"))
	})
}
