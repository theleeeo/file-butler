package server_test

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
	"github.com/theleeeo/file-butler/server"
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

func Test_CreateServer(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-all",
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := server.NewServer(server.Config{
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

func Test_Download(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-all",
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := server.NewServer(server.Config{
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

	prov.On("GetObject", mock.Anything, "123").Return(
		"hello",
		nil)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/mock/123", port), nil)
	assert.NoError(t, err)

	client := http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	d, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(d))
}

func Test_Upload(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-all",
	})
	assert.NoError(t, err)

	port, err := getValidPort()
	assert.NoError(t, err)

	srv, err := server.NewServer(server.Config{
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

	prov.On("PutObject", mock.Anything, "123", []byte("hello")).Return(nil)

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/mock/123", port), strings.NewReader("hello"))
	assert.NoError(t, err)

	client := http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
