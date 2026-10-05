package s3

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionOperationsWithoutConnection(t *testing.T) {
	client, err := NewClient(NewConfig(), nil)
	require.NoError(t, err)

	ctx := context.Background()
	bucket, err := client.Bucket("bucket")
	require.NoError(t, err)

	tests := []struct {
		name string
		call func() error
	}{
		{"PutObjectInfo", func() error {
			_, err := bucket.PutObjectInfo(ctx, "key", bytes.NewReader([]byte("x")), 1)
			return err
		}},
		{"CopyObjectVersion", func() error {
			_, err := bucket.CopyObjectVersion(ctx, CopySource{Name: "key"}, "bucket", "dst")
			return err
		}},
		{"SetObjectLegalHoldVersion", func() error {
			return bucket.SetObjectLegalHoldVersion(ctx, "key", "v1", true)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, ErrClientNotConnected, tt.call())
		})
	}
}

// A copy answered with "200 OK" and an error body decodes to an empty ETag in
// minio-go; CopyObjectVersion must report it as an error.
func TestCopyObjectVersionEmptyResult(t *testing.T) {
	type recorded struct {
		method string
		path   string
		copy   string
	}
	var mu sync.Mutex
	var reqs []recorded

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, recorded{r.Method, r.URL.Path, r.Header.Get("X-Amz-Copy-Source")})
		mu.Unlock()
		if r.Method == http.MethodPut && r.URL.Path == "/src-bucket/dst" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>InternalError</Code><Message>x</Message></Error>`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := NewConfig()
	cfg.Endpoint = strings.TrimPrefix(srv.URL, "http://")
	cfg.UseSSL = false
	cfg.ForcePathStyle = true
	cfg.Region = "us-east-1"
	cfg.AccessKeyID = "test"
	cfg.Password = "test"

	client, err := NewClient(cfg, nil)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, client.Connect(ctx))
	defer func() { _ = client.Close() }()

	bucket, err := client.Bucket("src-bucket")
	require.NoError(t, err)

	_, err = bucket.CopyObjectVersion(ctx, CopySource{Name: "src"}, "src-bucket", "dst")
	assert.ErrorIs(t, err, ErrEmptyCopyResult)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, reqs, 1)
	assert.Equal(t, http.MethodPut, reqs[0].method)
	assert.NotEmpty(t, reqs[0].copy)
}
