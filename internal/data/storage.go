package data

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

// StorageConfig holds S3/RustFS configuration
type StorageConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
	Region          string
}

// StorageClient wraps MinIO client for S3-compatible storage
type StorageClient struct {
	client *minio.Client
	bucket string
	log    *log.Helper
}

// NewStorageClient creates a new S3-compatible storage client
func NewStorageClient(ctx *bootstrap.Context) (*StorageClient, func(), error) {
	l := ctx.NewLoggerHelper("storage/data/asset-service")

	cfg := &StorageConfig{
		Endpoint:        getEnvOrDefault("ASSET_S3_ENDPOINT", "localhost:9000"),
		AccessKeyID:     getEnvOrDefault("ASSET_S3_ACCESS_KEY", "minioadmin"),
		SecretAccessKey: getEnvOrDefault("ASSET_S3_SECRET_KEY", "minioadmin"),
		Bucket:          getEnvOrDefault("ASSET_S3_BUCKET", "assets"),
		UseSSL:          getEnvOrDefault("ASSET_S3_USE_SSL", "false") == "true",
		Region:          getEnvOrDefault("ASSET_S3_REGION", "us-east-1"),
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		l.Errorf("failed to create MinIO client: %v", err)
		return nil, func() {}, err
	}

	// Ensure bucket exists
	bgCtx := context.Background()
	exists, err := client.BucketExists(bgCtx, cfg.Bucket)
	if err != nil {
		l.Warnf("failed to check bucket existence: %v", err)
	} else if !exists {
		err = client.MakeBucket(bgCtx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region})
		if err != nil {
			l.Warnf("failed to create bucket: %v", err)
		} else {
			l.Infof("created bucket: %s", cfg.Bucket)
		}
	}

	sc := &StorageClient{
		client: client,
		bucket: cfg.Bucket,
		log:    l,
	}

	return sc, func() {}, nil
}

// UploadResult contains the result of an upload operation
type UploadResult struct {
	Key      string
	Size     int64
	Checksum string
}

// Upload uploads a file to storage
func (s *StorageClient) Upload(ctx context.Context, key string, content []byte, mimeType string) (*UploadResult, error) {
	hash := sha256.Sum256(content)
	checksum := hex.EncodeToString(hash[:])

	reader := bytes.NewReader(content)
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(content)), minio.PutObjectOptions{
		ContentType: mimeType,
		UserMetadata: map[string]string{
			"checksum": checksum,
		},
	})
	if err != nil {
		s.log.Errorf("failed to upload file: %v", err)
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{
		Key:      key,
		Size:     int64(len(content)),
		Checksum: checksum,
	}, nil
}

// Download downloads a file from storage
func (s *StorageClient) Download(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		s.log.Errorf("failed to get object: %v", err)
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	content, err := io.ReadAll(obj)
	if err != nil {
		s.log.Errorf("failed to read object: %v", err)
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return content, nil
}

// Delete deletes a file from storage
func (s *StorageClient) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		s.log.Errorf("failed to delete object: %v", err)
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// GetPresignedURL generates a presigned URL for downloading
func (s *StorageClient) GetPresignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiresIn, nil)
	if err != nil {
		s.log.Errorf("failed to generate presigned URL: %v", err)
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}
