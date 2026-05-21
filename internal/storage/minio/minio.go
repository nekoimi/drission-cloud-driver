package minio

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	minioClient "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/nekoimi/go-project-template/internal/config"
	"github.com/nekoimi/go-project-template/internal/pkg/snowflake"
	"github.com/nekoimi/go-project-template/internal/storage"
)

type minioStorage struct {
	client    *minioClient.Client
	bucket    string
	publicURL string
}

func New(cfg config.StorageConfig) (storage.FileStorage, error) {
	client, err := minioClient.New(cfg.Minio.Endpoint, &minioClient.Options{
		Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
		Secure: cfg.Minio.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Minio.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Minio.Bucket, minioClient.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &minioStorage{
		client:    client,
		bucket:    cfg.Minio.Bucket,
		publicURL: cfg.Minio.PublicURL,
	}, nil
}

func sanitizeMinIOFolder(folder string) (string, error) {
	raw := strings.ReplaceAll(folder, "\\", "/")
	for _, p := range strings.Split(raw, "/") {
		if p == ".." {
			return "", fmt.Errorf("invalid folder path")
		}
	}
	out := filepath.ToSlash(filepath.Clean(raw))
	return strings.Trim(out, "/"), nil
}

func (s *minioStorage) Upload(ctx context.Context, file *storage.FileHeader, folder string) (*storage.UploadResult, error) {
	folder, err := sanitizeMinIOFolder(folder)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(file.Filename)
	filename := snowflake.GenerateStringID() + ext
	objectName := filename
	if folder != "" {
		objectName = folder + "/" + filename
	}

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = s.client.PutObject(ctx, s.bucket, objectName, file.File, file.Size, minioClient.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to minio: %w", err)
	}

	return &storage.UploadResult{
		Path:     objectName,
		URL:      s.GetURL(objectName),
		Size:     file.Size,
		MimeType: contentType,
	}, nil
}

func (s *minioStorage) Delete(ctx context.Context, path string) error {
	return s.client.RemoveObject(ctx, s.bucket, path, minioClient.RemoveObjectOptions{})
}

func (s *minioStorage) GetURL(path string) string {
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.publicURL, "/"), s.bucket, path)
}

func (s *minioStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, path, minioClient.StatObjectOptions{})
	if err != nil {
		errResp := minioClient.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
