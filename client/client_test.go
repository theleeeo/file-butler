package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Upload(t *testing.T) {

	t.Run("ok, tags", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/some/path/filename.jpg", r.URL.Path)
			assert.Equal(t, "tag1value", r.URL.Query().Get("tag1"))
			assert.Equal(t, "tag2value", r.URL.Query().Get("tag2"))
		}))
		defer mockServer.Close()

		client, err := New(mockServer.URL, 0)
		if err != nil {
			t.Fatalf("cannot create client: %v", err)
		}

		err = client.Upload(context.Background(), []byte("data"), "some/path", "filename.jpg", map[string]string{"tag1": "tag1value", "tag2": "tag2value"})
		assert.NoError(t, err)
	})

	t.Run("should fail", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer mockServer.Close()

		client, err := New(mockServer.URL, 0)
		if err != nil {
			t.Fatalf("cannot create client: %v", err)
		}

		err = client.Upload(context.Background(), []byte("data"), "some/path", "filename.jpg", nil)
		assert.Error(t, err)
	})

	t.Run("ok, no tags", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/some/path/filename.jpg", r.URL.Path)
		}))
		defer mockServer.Close()

		client, err := New(mockServer.URL, 0)
		if err != nil {
			t.Fatalf("cannot create client: %v", err)
		}

		err = client.Upload(context.Background(), []byte("data"), "some/path", "filename.jpg", nil)
		assert.NoError(t, err)
	})

	t.Run("ok, weird tags", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/some/path/filename.jpg", r.URL.Path)
			assert.Equal(t, "¨¨¨'!#$%&/()", r.URL.Query().Get("åääööä"))
		}))
		defer mockServer.Close()

		client, err := New(mockServer.URL, 0)
		if err != nil {
			t.Fatalf("cannot create client: %v", err)
		}

		err = client.Upload(context.Background(), []byte("data"), "some/path", "filename.jpg", map[string]string{"åääööä": "¨¨¨'!#$%&/()"})
		assert.NoError(t, err)
	})

}
