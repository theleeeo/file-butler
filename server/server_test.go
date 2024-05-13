package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	authPlugin "github.com/theleeeo/file-butler/authorization/plugin"
	"github.com/theleeeo/file-butler/server"
)

func Test_CreateServer(t *testing.T) {
	plg, err := authPlugin.NewPlugin(authPlugin.Config{
		Name:    "default",
		BuiltIn: "allow-all",
	})
	assert.NoError(t, err)

	srv, err := server.NewServer(server.Config{
		Addr:              "localhost:0",
		DefaultAuthPlugin: "default",
	}, []authPlugin.Plugin{plg})
	assert.NoError(t, err)
	assert.NotNil(t, srv)
}
