package storage

import "mime/multipart"

type FileHeader struct {
	File     multipart.File
	Header   *multipart.FileHeader
	Filename string
	Size     int64
}

type UploadResult struct {
	Path     string `json:"path"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}
