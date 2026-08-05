package storage

import "io"

// PresignedResult 预签名上传结果
type PresignedResult struct {
	UploadURL string `json:"upload_url"` // 前端直传 URL
	ObjectKey string `json:"object_key"` // 存储中的对象键
	PublicURL string `json:"public_url"` // 上传后的公开访问 URL
}

// Provider 存储提供者接口
type Provider interface {
	// PresignUpload 生成预签名上传 URL
	// key: 对象键, contentType: MIME类型, maxSize: 最大字节数
	// S3 模式返回带 UploadURL 的直传地址；本地模式仅返回 ObjectKey，需走中转上传
	PresignUpload(key string, contentType string, maxSize int64) (*PresignedResult, error)

	// UploadFromReader 从 reader 读取数据上传（本地模式用）
	UploadFromReader(key string, reader io.Reader, size int64, contentType string) (publicURL string, err error)

	// GetPublicURL 获取对象的公开访问 URL
	GetPublicURL(key string) string

	// Delete 删除对象
	Delete(key string) error

	// TestConnection 校验存储后端连接是否可用
	TestConnection() error

	// HeadObjectSize 获取对象大小（字节），用于上传完成后的服务端复核
	HeadObjectSize(key string) (int64, error)
}
