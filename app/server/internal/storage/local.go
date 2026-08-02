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

// PresignUpload 本地模式无预签名直传，返回 object_key 供前端走中转上传（/storage/upload）。
func (p *LocalProvider) PresignUpload(key string, contentType string, maxSize int64) (*PresignedResult, error) {
	return &PresignedResult{ObjectKey: key}, nil
}

// TestConnection 本地存储无需额外连接校验。
func (p *LocalProvider) TestConnection() error {
	return nil
}

// resolvePath 清理 key 并拼接 basePath，拒绝包含 ../ 的路径穿越。
// 返回绝对全路径与 basePath 绝对路径，供调用方二次校验。
func (p *LocalProvider) resolvePath(key string) (string, string, error) {
	if key == "" {
		return "", "", fmt.Errorf("empty object key")
	}
	// 以 "/" 为根做 Clean，剥离所有 .. 与多余分隔符，避免逃逸 basePath。
	cleaned := filepath.Clean("/" + key)
	full := filepath.Join(p.basePath, cleaned)
	absBase, err := filepath.Abs(p.basePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve base path failed: %w", err)
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", "", fmt.Errorf("resolve full path failed: %w", err)
	}
	if absFull != absBase && !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) {
		return "", "", fmt.Errorf("invalid object key: path traversal detected")
	}
	return full, absBase, nil
}

// UploadFromReader 从 reader 读取数据写入本地文件
func (p *LocalProvider) UploadFromReader(key string, reader io.Reader, size int64, contentType string) (string, error) {
	fullPath, _, err := p.resolvePath(key)
	if err != nil {
		return "", err
	}
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
	fullPath, _, err := p.resolvePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file failed: %w", err)
	}
	return nil
}
