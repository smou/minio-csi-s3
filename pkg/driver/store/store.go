package store

import (
	"context"
	"net/url"

	"k8s.io/klog/v2"
)

type BucketStore interface {
	// BucketExists prüft, ob der Bucket existiert
	BucketExists(ctx context.Context, name string) (bool, error)

	// CreateBucket erstellt den Bucket, falls er nicht existiert
	CreateBucket(ctx context.Context, name string) error

	// DeleteBucket löscht den Bucket, falls er existiert
	DeleteBucket(ctx context.Context, name string) error
}

type StoreConfig struct {
	EndpointURL string
	Region      string
	AccessKey   string
	SecretKey   string
}

func (c *StoreConfig) UseTLS() bool {
	u, err := url.Parse(c.EndpointURL)
	if err != nil || u.Host == "" {
		klog.Warningf("Error determine TLS: %v", err)
		return true
	}
	return u.Scheme == "https"
}

func (c *StoreConfig) Endpoint() string {
	u, err := url.Parse(c.EndpointURL)
	if err != nil || u.Host == "" {
		klog.Warningf("Error parse EndpointURL: %v", err)
		return c.EndpointURL
	}
	return u.Host
}
