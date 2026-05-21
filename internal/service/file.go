package service

import (
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/nekoimi/go-project-template/internal/storage"
)

type FileService interface {
	UploadSingle(ctx context.Context, file *multipart.FileHeader, folder string) (*storage.UploadResult, error)
	UploadMultiple(ctx context.Context, files []*multipart.FileHeader, folder string) ([]*storage.UploadResult, error)
}

type fileService struct {
	storage      storage.FileStorage
	allowedExts  map[string]bool
	allowedMIMEs map[string]bool
}

func NewFileService(storage storage.FileStorage, allowedExts []string, allowedMIMEs []string) FileService {
	extMap := make(map[string]bool, len(allowedExts))
	for _, ext := range allowedExts {
		extMap[strings.ToLower(ext)] = true
	}
	mimeMap := make(map[string]bool, len(allowedMIMEs))
	for _, m := range allowedMIMEs {
		mimeMap[strings.ToLower(m)] = true
	}
	return &fileService{
		storage:      storage,
		allowedExts:  extMap,
		allowedMIMEs: mimeMap,
	}
}

func (s *fileService) validateFile(fileHeader *multipart.FileHeader) error {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if len(s.allowedExts) > 0 && !s.allowedExts[ext] {
		return fmt.Errorf("file extension %q not allowed", ext)
	}

	// Detect actual MIME type from file content
	f, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open file for validation: %w", err)
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read file header: %w", err)
	}

	detectedMIME := http.DetectContentType(buf[:n])
	mediaType, _, err := mime.ParseMediaType(detectedMIME)
	if err != nil {
		mediaType = detectedMIME
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if len(s.allowedMIMEs) > 0 && !s.allowedMIMEs[mediaType] {
		return fmt.Errorf("file MIME type %q not allowed", mediaType)
	}

	return nil
}

func (s *fileService) UploadSingle(ctx context.Context, fileHeader *multipart.FileHeader, folder string) (*storage.UploadResult, error) {
	if err := s.validateFile(fileHeader); err != nil {
		return nil, err
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	fh := &storage.FileHeader{
		File:     file,
		Header:   fileHeader,
		Filename: fileHeader.Filename,
		Size:     fileHeader.Size,
	}

	return s.storage.Upload(ctx, fh, folder)
}

func (s *fileService) UploadMultiple(ctx context.Context, files []*multipart.FileHeader, folder string) ([]*storage.UploadResult, error) {
	var results []*storage.UploadResult

	for _, fileHeader := range files {
		if err := s.validateFile(fileHeader); err != nil {
			return results, fmt.Errorf("%s: %w", fileHeader.Filename, err)
		}

		result, err := s.UploadSingle(ctx, fileHeader, folder)
		if err != nil {
			return results, fmt.Errorf("failed to upload %s: %w", fileHeader.Filename, err)
		}
		results = append(results, result)
	}

	return results, nil
}
