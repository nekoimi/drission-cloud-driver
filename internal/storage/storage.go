package storage

import "context"

type FileStorage interface {
	Upload(ctx context.Context, file *FileHeader, folder string) (*UploadResult, error)
	Delete(ctx context.Context, path string) error
	GetURL(path string) string
	Exists(ctx context.Context, path string) (bool, error)
}
