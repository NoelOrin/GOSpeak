package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// StorageHandler 存储相关 handler
type StorageHandler struct {
	storageService *service.StorageService
}

// NewStorageHandler 创建存储 handler
func NewStorageHandler(storageService *service.StorageService) *StorageHandler {
	return &StorageHandler{storageService: storageService}
}

// categoryPattern 上传分类白名单：字母/数字/下划线/短横线，1-32 位。
var categoryPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

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

	if !categoryPattern.MatchString(req.Category) {
		pkg.Fail(c, pkg.INVALID_PARAMS, "category is invalid")
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
	userUUID := currentUserUUID(c)
	if userUUID == "" {
		pkg.Fail(c, pkg.TOKEN_WRONG, "user_uuid is required")
		return
	}
	// 对象键绑定当前用户，防止上传阶段覆盖他人对象。
	objectKey := generateObjectKey(cfg.PathPrefix, req.Category, req.FileName, userUUID)

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

	// 获取配置用于所有权与大小复核
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 对象键必须属于当前用户，防止确认/查询他人对象。
	userUUID := currentUserUUID(c)
	if userUUID == "" {
		pkg.Fail(c, pkg.TOKEN_WRONG, "user_uuid is required")
		return
	}
	if !strings.HasPrefix(req.ObjectKey, objectKeyUserPrefix(cfg.PathPrefix, userUUID)) {
		pkg.Fail(c, pkg.FORBIDDEN, "object_key does not belong to current user")
		return
	}

	// 获取 provider 拿到公开 URL
	p, err := h.storageService.GetProvider()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// S3 直传绕过中转，确认时用 HeadObject 复核实际大小，防止绕过 MaxFileSize。
	if cfg.ProviderType == "s3" {
		size, err := p.HeadObjectSize(req.ObjectKey)
		if err != nil {
			pkg.Fail(c, pkg.STORAGE_ERROR)
			return
		}
		maxBytes := int64(cfg.MaxFileSize) * 1024 * 1024
		if cfg.MaxFileSize <= 0 {
			maxBytes = 5 * 1024 * 1024
		}
		if size > maxBytes {
			pkg.Fail(c, pkg.STORAGE_FILE_TOO_LARGE)
			return
		}
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
	// 获取配置用于校验
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 请求体上限：文件大小 + multipart 开销余量，防止超大 body 整段进入内存/临时盘。
	maxMB := cfg.MaxFileSize
	if maxMB <= 0 {
		maxMB = 5
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxMB)*1024*1024+1024)

	file, err := c.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			pkg.Fail(c, pkg.STORAGE_FILE_TOO_LARGE)
			return
		}
		pkg.Fail(c, pkg.INVALID_PARAMS, "file is required")
		return
	}

	objectKey := c.PostForm("object_key")
	if objectKey == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "object_key is required")
		return
	}

	// 本地中转上传必须使用当前用户 presign 时生成的对象键，防止覆盖他人对象。
	userUUID := currentUserUUID(c)
	if userUUID == "" || !strings.HasPrefix(objectKey, objectKeyUserPrefix(cfg.PathPrefix, userUUID)) {
		pkg.Fail(c, pkg.FORBIDDEN, "object_key does not belong to current user")
		return
	}

	// 文件大小校验
	maxBytes := int64(cfg.MaxFileSize) * 1024 * 1024
	if cfg.MaxFileSize <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
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
	contentType := normalizeContentType(pkg.DetectContentType(sniff[:n]))
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
