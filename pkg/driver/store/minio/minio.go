package minio

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/smou/k8s-csi-s3/pkg/driver/store"
	"k8s.io/klog/v2"
)

type Store struct {
	Client *minio.Client
	Region string
}

func NewStore(config *store.StoreConfig) (*Store, error) {
	klog.Infof("Init MinioStore | '%s' | '%s'", config.EndpointURL, config.Region)
	client, err := minio.New(config.Endpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseTLS(),
		Region: config.Region,
	})
	if err != nil {
		return nil, err
	}
	return &Store{
		Client: client,
		Region: config.Region,
	}, nil
}

func (s *Store) BucketExists(ctx context.Context, name string) (bool, error) {
	klog.Infof("BucketExists? '%s'", name)
	exists, err := s.Client.BucketExists(ctx, name)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) CreateBucket(ctx context.Context, name string) error {
	klog.Infof("CreateBucket '%s'", name)
	exists, err := s.BucketExists(ctx, name)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return s.Client.MakeBucket(ctx, name, minio.MakeBucketOptions{
		Region: s.Region,
	})
}

func (s *Store) DeleteBucket(ctx context.Context, name string) error {
	klog.Infof("DeleteBucket '%s'", name)
	exists, err := s.BucketExists(ctx, name)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}
	return s.Client.RemoveBucketWithOptions(ctx, name, minio.RemoveBucketOptions{
		ForceDelete: true,
	})
}
