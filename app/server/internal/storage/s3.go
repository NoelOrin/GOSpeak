package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Provider S3 兼容存储提供者
type S3Provider struct {
	client        *s3.Client
	presigner     *s3.PresignClient
	endpoint      string
	bucket        string
	region        string
	pathPrefix    string
	publicBaseURL string
}

// NewS3Provider 创建 S3 存储提供者
func NewS3Provider(cfg model.StorageConfig) (*S3Provider, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	// 解析 endpoint，去除路径前缀以确保正确格式
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid s3 endpoint: %w", err)
	}

	isPathStyle := strings.Contains(endpointURL.Host, "localhost") ||
		strings.Contains(endpointURL.Host, "127.0.0.1") ||
		!strings.HasSuffix(endpointURL.Host, "amazonaws.com")

	customResolver := aws.EndpointResolverWithOptionsFunc(func(_, _ string, _ ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			SigningRegion:     region,
			HostnameImmutable: !isPathStyle,
		}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config failed: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if isPathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	presigner := s3.NewPresignClient(client)

	return &S3Provider{
		client:        client,
		presigner:     presigner,
		endpoint:      endpoint,
		bucket:        cfg.Bucket,
		region:        region,
		pathPrefix:    cfg.PathPrefix,
		publicBaseURL: cfg.PublicBaseURL,
	}, nil
}

// Name 返回提供者名称
func (p *S3Provider) Name() string {
	return "s3"
}

// PresignUpload 生成预签名 PUT URL
func (p *S3Provider) PresignUpload(key string, contentType string, maxSize int64) (*PresignedResult, error) {
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	presignedReq, err := p.presigner.PresignPutObject(context.Background(), putInput, func(opts *s3.PresignOptions) {
		opts.Expires = 5 * time.Minute
	})
	if err != nil {
		return nil, fmt.Errorf("presign upload failed: %w", err)
	}

	return &PresignedResult{
		UploadURL: presignedReq.URL,
		ObjectKey: key,
		PublicURL: p.GetPublicURL(key),
	}, nil
}

// UploadFromReader 通过 S3 PutObject 上传
func (p *S3Provider) UploadFromReader(key string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := p.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", fmt.Errorf("put object failed: %w", err)
	}

	return p.GetPublicURL(key), nil
}

// GetPublicURL 拼接公开访问 URL
func (p *S3Provider) GetPublicURL(key string) string {
	base := strings.TrimRight(p.publicBaseURL, "/")
	if base != "" {
		return base + "/" + key
	}

	// 智能拼接：
	// AWS S3: https://{bucket}.s3.{region}.amazonaws.com/{key}
	// 其他（MinIO/R2 等）: {endpoint}/{bucket}/{key}
	ep := strings.TrimRight(p.endpoint, "/")
	if strings.HasSuffix(ep, ".amazonaws.com") {
		// endpoint 格式: https://s3.{region}.amazonaws.com → 提取 region
		host := strings.TrimPrefix(strings.TrimPrefix(ep, "https://"), "http://")
		host = strings.TrimSuffix(host, ".amazonaws.com")
		parts := strings.Split(host, ".")
		region := ""
		if len(parts) >= 2 {
			region = parts[1]
		}
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", p.bucket, region, key)
	}
	// path-style: endpoint/bucket/key
	return fmt.Sprintf("%s/%s/%s", ep, p.bucket, key)
}

// Delete 删除 S3 对象
func (p *S3Provider) Delete(key string) error {
	_, err := p.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object failed: %w", err)
	}
	return nil
}
