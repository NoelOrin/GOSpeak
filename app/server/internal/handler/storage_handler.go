package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StorageHandler 存储相关 handler
type StorageHandler struct {
	storageService *service.StorageService
}

// NewStorageHandler 创建存储 handler
func NewStorageHandler(storageService *service.StorageService) *StorageHandler {
	return &StorageHandler{storageService: storageService}
}

// presignUploadRequest 预签名上传请求
type presignUploadRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required"`
	Category    string `json:"category" binding:"required"`
}

// presignUploadResponse 预签名上传响应
type presignUploadResponse struct {
	ProviderType string  `json:"provider_type"`
	UploadURL    *string `json:"upload_url"`
	ObjectKey    string  `json:"object_key"`
	PublicURL    string  `json:"public_url,omitempty"`
}

// confirmUploadRequest 确认上传请求
type confirmUploadRequest struct {
	ObjectKey string `json:"object_key" binding:"required"`
}

// PresignUpload
// @Summary      获取预签名上传 URL
// @Description  获取文件上传的预签名 URL（S3 模式直传，本地模式返回 object_key）
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  presignUploadRequest  true  "上传请求"
// @Success      200   {object}  pkg.Response
// @Router       /storage/presign [post]
func (h *StorageHandler) PresignUpload(c *gin.Context) {
	var req presignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	// 获取配置用于校验
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 文件大小校验
	maxBytes := int64(cfg.MaxFileSize) * 1024 * 1024
	if req.FileSize > maxBytes {
		pkg.Fail(c, pkg.STORAGE_FILE_TOO_LARGE)
		return
	}

	// 文件类型校验

	contentType := normalizeContentType(req.ContentType)
	if !isAllowedType(contentType, effectiveAllowedTypes(cfg.AllowedTypes)) || !isSafeFileExtension(filepath.Ext(req.FileName), contentType) {
		pkg.Fail(c, pkg.STORAGE_FILE_TYPE_NOT_ALLOWED)
		return
	}

	// 生成对象键：{path_prefix}{category}/{uuid}.{ext}
	objectKey := generateObjectKey(cfg.PathPrefix, req.Category, req.FileName)

	// 获取预签名 URL
	result, err := h.storageService.PresignUpload(objectKey, contentType, maxBytes)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	resp := presignUploadResponse{
		ProviderType: cfg.ProviderType,
		ObjectKey:    result.ObjectKey,
	}
	if result != nil && result.UploadURL != "" {
		resp.UploadURL = &result.UploadURL
		resp.PublicURL = result.PublicURL
	}

	pkg.Success(c, resp)
}

// ConfirmUpload
// @Summary      确认 S3 上传完成
// @Description  S3 直传完成后调用，确认文件已上传并返回公开 URL
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  confirmUploadRequest  true  "确认请求"
// @Success      200   {object}  pkg.Response
// @Router       /storage/confirm [post]
func (h *StorageHandler) ConfirmUpload(c *gin.Context) {
	var req confirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	// 获取 provider 拿到公开 URL
	p, err := h.storageService.GetProvider()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	publicURL := p.GetPublicURL(req.ObjectKey)
	pkg.Success(c, gin.H{"public_url": publicURL})
}

// Upload
// @Summary      本地模式中转上传
// @Description  本地存储模式下通过服务器中转上传文件
// @Tags         存储
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file        formData  file    true  "上传文件"
// @Param        object_key  formData  string  true  "对象键"
// @Success      200  {object}  pkg.Response
// @Router       /storage/upload [post]
func (h *StorageHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "file is required")
		return
	}

	objectKey := c.PostForm("object_key")
	if objectKey == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "object_key is required")
		return
	}

	// 获取配置用于校验
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 文件大小校验
	maxBytes := int64(cfg.MaxFileSize) * 1024 * 1024
	if file.Size > maxBytes {
		pkg.Fail(c, pkg.STORAGE_FILE_TOO_LARGE)
		return
	}

	// 打开上传文件
	src, err := file.Open()
	if err != nil {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "failed to open uploaded file")
		return
	}
	defer src.Close()

	// 不信任客户端 Content-Type，使用文件头 Magic Bytes 嗅探后校验。
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(src, sniff)
	contentType := normalizeContentType(http.DetectContentType(sniff[:n]))
	if !isAllowedType(contentType, effectiveAllowedTypes(cfg.AllowedTypes)) || !isSafeFileExtension(filepath.Ext(objectKey), contentType) {
		pkg.Fail(c, pkg.STORAGE_FILE_TYPE_NOT_ALLOWED)
		return
	}
	if _, err = src.Seek(0, io.SeekStart); err != nil {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "failed to reset upload reader")
		return
	}

	publicURL, err := h.storageService.UploadFromReader(objectKey, src, file.Size, contentType)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"public_url": publicURL})
}

// DeleteObject
// @Summary      删除存储对象
// @Description  管理员删除存储中的文件
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{key=string}  true  "对象键"
// @Success      200   {object}  pkg.Response
// @Router       /storage/delete [post]
func (h *StorageHandler) DeleteObject(c *gin.Context) {
	var req struct {
		Key string `json:"key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.storageService.DeleteObject(req.Key); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}

// GetConfig
// @Summary      获取存储配置
// @Description  管理员获取存储配置（AK/SK 脱敏）
// @Tags         存储
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /storage/config [post]
func (h *StorageHandler) GetConfig(c *gin.Context) {
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, service.ToPublicStorageConfig(cfg))
}

// UpdateConfig
// @Summary      更新存储配置
// @Description  管理员更新存储配置
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.UpdateStorageConfigRequest  true  "配置请求"
// @Success      200   {object}  pkg.Response
// @Router       /storage/update-config [post]
func (h *StorageHandler) UpdateConfig(c *gin.Context) {
	var req service.UpdateStorageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	cfg, err := h.storageService.UpdateConfigFromDTO(req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, cfg)
}

// generateObjectKey 生成对象键
func generateObjectKey(pathPrefix, category, fileName string) string {
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	id := uuid.New().String()
	if pathPrefix == "" {
		pathPrefix = "uploads/"
	}
	// 确保 pathPrefix 以 / 结尾
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}
	return pathPrefix + category + "/" + id + ext
}

// normalizeContentType 去掉 MIME 后的 charset 等参数。
func normalizeContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	return contentType
}

// effectiveAllowedTypes 在允许列表为空时回退到安全默认值，不再全类型放行。
func effectiveAllowedTypes(allowedTypes string) string {
	if strings.TrimSpace(allowedTypes) == "" {
		return "image/jpeg,image/png,image/gif,image/webp,application/pdf,text/plain"
	}
	return allowedTypes
}

// isAllowedType 检查 MIME 类型是否在允许列表中，支持 image/* 等通配。
func isAllowedType(contentType, allowedTypes string) bool {
	contentType = normalizeContentType(contentType)
	if contentType == "" {
		return false
	}
	for _, t := range strings.Split(effectiveAllowedTypes(allowedTypes), ",") {
		allowed := normalizeContentType(t)
		if allowed == contentType || allowed == "*/*" {
			return true
		}
		if strings.HasSuffix(allowed, "/*") && strings.HasPrefix(contentType, strings.TrimSuffix(allowed, "*")) {
			return true
		}
	}
	return false
}

// isSafeFileExtension 防止用户以 .html/.svg 等危险扩展名伪装成安全 MIME。
func isSafeFileExtension(ext, contentType string) bool {
	contentType = normalizeContentType(contentType)
	extension := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	switch contentType {
	case "image/jpeg":
		return extension == "jpg" || extension == "jpeg"
	case "image/png":
		return extension == "png"
	case "image/gif":
		return extension == "gif"
	case "image/webp":
		return extension == "webp"
	case "application/pdf":
		return extension == "pdf"
	case "text/plain":
		return extension == "txt" || extension == "md" || extension == "csv" || extension == "log"
	default:
		return false
	}
}
