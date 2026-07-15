package storage

import (
	"context"
	"io"
)

type FileStorage interface {
	UploadFile(ctx context.Context, bucketName, objectKey string, reader io.Reader) (string, error)
	GetSignedURL(ctx context.Context, bucketName, objectKey string) (string, error)
}

type s3Storage struct {
	// AWS SDK Client session
}

func NewS3Storage() FileStorage {
	return &s3Storage{}
}

func (s *s3Storage) UploadFile(ctx context.Context, bucketName, objectKey string, reader io.Reader) (string, error) {
	// TODO: Upload object to S3 bucket and return SHA-256 hash / file path
	return "", nil
}

func (s *s3Storage) GetSignedURL(ctx context.Context, bucketName, objectKey string) (string, error) {
	// TODO: Generate pre-signed URL for client decyption
	return "", nil
}
