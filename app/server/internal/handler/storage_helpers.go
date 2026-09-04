package handler

import (
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// generateObjectKey 生成对象键
func generateObjectKey(pathPrefix, category, fileName, userUUID string) string {
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
	return pathPrefix + userUUID + "/" + category + "/" + id + ext
}

// objectKeyUserPrefix 返回当前用户 presign 对象键的前缀，用于上传所有权校验。
func objectKeyUserPrefix(pathPrefix, userUUID string) string {
	if pathPrefix == "" {
		pathPrefix = "uploads/"
	}
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}
	return pathPrefix + userUUID + "/"
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
