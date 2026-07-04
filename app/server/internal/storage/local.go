package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalProvider 本地文件系统存储
type LocalProvider struct {
	basePath      string // 文件存储根路径
	urlPrefix     string // URL 前缀
	publicBaseURL string // 公开访问基础 URL（可选，覆盖 urlPrefix）
}

// NewLocalProvider 创建本地存储提供者
func NewLocalProvider(basePath string, urlPrefix string, publicBaseURL string) *LocalProvider {
	if basePath == "" {
		basePath = "./uploads"
	}
	if urlPrefix == "" {
		urlPrefix = "/uploads"
	}
	return &LocalProvider{basePath: basePath, urlPrefix: urlPrefix, publicBaseURL: publicBaseURL}
}

// Name 返回提供者名称
func (p *LocalProvider) Name() string {
	return "local"
}

// PresignUpload 本地模式不支持预签名，返回 nil
func (p *LocalProvider) PresignUpload(key string, contentType string, maxSize int64) (*PresignedResult, error) {
	return nil, nil
}

// UploadFromReader 从 reader 读取数据写入本地文件
func (p *LocalProvider) UploadFromReader(key string, reader io.Reader, size int64, contentType string) (string, error) {
	fullPath := filepath.Join(p.basePath, key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory failed: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}

	return p.GetPublicURL(key), nil
}

// GetPublicURL 拼接公开访问 URL
func (p *LocalProvider) GetPublicURL(key string) string {
	if base := strings.TrimRight(p.publicBaseURL, "/"); base != "" {
		return base + "/" + key
	}
	return p.urlPrefix + "/" + key
}

// Delete 删除本地文件
func (p *LocalProvider) Delete(key string) error {
	fullPath := filepath.Join(p.basePath, key)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file failed: %w", err)
	}
	return nil
}
