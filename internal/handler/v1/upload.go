package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/response"
	"github.com/nekoimi/go-project-template/internal/service"
)

type UploadHandler struct {
	fileService service.FileService
	logger      *zap.Logger
}

func NewUploadHandler(fileService service.FileService, logger *zap.Logger) *UploadHandler {
	return &UploadHandler{fileService: fileService, logger: logger}
}

// UploadSingle godoc
// @Summary      Upload a single file
// @Tags         upload
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file   formData  file    true  "File to upload"
// @Param        folder formData  string  false "Upload folder"
// @Success      200    {object}  response.APIResponse
// @Failure      400    {object}  response.APIResponse
// @Router       /upload/single [post]
func (h *UploadHandler) UploadSingle(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.ErrorWithMsg(c, http.StatusBadRequest, errcode.BadRequest, "missing file")
		return
	}

	folder := c.DefaultPostForm("folder", "uploads")

	result, err := h.fileService.UploadSingle(c.Request.Context(), file, folder)
	if err != nil {
		h.logger.Error("upload single failed", zap.Error(err))
		response.ErrorWithMsg(c, http.StatusInternalServerError, errcode.Internal, "upload failed")
		return
	}

	response.Success(c, result)
}

// UploadMultiple godoc
// @Summary      Upload multiple files
// @Tags         upload
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        files  formData  []file  true  "Files to upload"
// @Param        folder formData  string  false "Upload folder"
// @Success      200    {object}  response.APIResponse
// @Failure      400    {object}  response.APIResponse
// @Router       /upload/multiple [post]
func (h *UploadHandler) UploadMultiple(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		response.ErrorWithMsg(c, http.StatusBadRequest, errcode.BadRequest, "invalid multipart form")
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		response.ErrorWithMsg(c, http.StatusBadRequest, errcode.BadRequest, "no files provided")
		return
	}

	folder := c.DefaultPostForm("folder", "uploads")

	results, err := h.fileService.UploadMultiple(c.Request.Context(), files, folder)
	if err != nil {
		h.logger.Error("upload multiple failed", zap.Error(err))
		response.ErrorWithMsg(c, http.StatusInternalServerError, errcode.Internal, "upload failed")
		return
	}

	response.Success(c, results)
}
