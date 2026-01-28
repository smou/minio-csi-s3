package store_test

import (
	"testing"

	"github.com/smou/k8s-csi-s3/pkg/driver/store"
)

func TestStoreConfig_EndpointAndTLS(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantHost string
		wantTLS  bool
	}{
		{
			name:     "https endpoint",
			endpoint: "https://minio.example.com:9000",
			wantHost: "minio.example.com:9000",
			wantTLS:  true,
		},
		{
			name:     "http endpoint",
			endpoint: "http://127.0.0.1:9000",
			wantHost: "127.0.0.1:9000",
			wantTLS:  false,
		},
		{
			name:     "endpoint without scheme",
			endpoint: "minio:9000",
			wantHost: "minio:9000",
			wantTLS:  true, // default fallback
		},
		{
			name:     "invalid url",
			endpoint: "://invalid-url",
			wantHost: "://invalid-url",
			wantTLS:  true, // safe default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &store.StoreConfig{
				EndpointURL: tt.endpoint,
			}

			if got := cfg.Endpoint(); got != tt.wantHost {
				t.Fatalf("Endpoint() = %q, want %q", got, tt.wantHost)
			}

			if got := cfg.UseTLS(); got != tt.wantTLS {
				t.Fatalf("UseTLS() = %v, want %v", got, tt.wantTLS)
			}
		})
	}
}
