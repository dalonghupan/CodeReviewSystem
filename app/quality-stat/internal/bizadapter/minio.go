// Package bizadapter quality-stat 第三方适配层
// MinIO 对象存储客户端封装（报表文件存储，SRS F05-03）
package bizadapter

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"cr-system/pkg/errcode"
)

// MinioConfig MinIO 客户端配置
type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	Timeout   time.Duration
}

// MinioClient MinIO 对象存储客户端
type MinioClient struct {
	client *minio.Client
	bucket string
}

// NewMinioClient 创建 MinIO 客户端
func NewMinioClient(cfg MinioConfig) (*MinioClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, errcode.ErrMinIOUploadFailed.WithDetail("创建MinIO客户端失败: " + err.Error())
	}

	// 确保 bucket 存在
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, errcode.ErrMinIOUploadFailed.WithDetail("检查Bucket失败: " + err.Error())
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, errcode.ErrMinIOUploadFailed.WithDetail("创建Bucket失败: " + err.Error())
		}
	}

	return &MinioClient{client: client, bucket: cfg.Bucket}, nil
}

// UploadFile 上传文件到 MinIO
// objectKey 如 "reports/monthly/2026-08/xxx.xlsx"
// contentType 如 "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
func (m *MinioClient) UploadFile(ctx context.Context, objectKey, contentType string, data []byte) (string, error) {
	reader := bytes.NewReader(data)
	info, err := m.client.PutObject(ctx, m.bucket, objectKey, reader, int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", errcode.ErrMinIOUploadFailed.WithDetail("上传文件失败: " + err.Error())
	}

	// 构造文件访问 URL
	url := fmt.Sprintf("%s/%s/%s", m.client.EndpointURL().String(), m.bucket, objectKey)
	_ = info // info 含 ETag 等元信息

	return url, nil
}

// GetDownloadURL 获取预签名下载 URL（过期时间 1 小时）
func (m *MinioClient) GetDownloadURL(ctx context.Context, objectKey string) (string, time.Time, error) {
	expiry := 1 * time.Hour
	reqParams := url.Values{}
	reqParams.Set("response-content-disposition", "attachment")

	presignedURL, err := m.client.PresignedGetObject(ctx, m.bucket, objectKey, expiry, reqParams)
	if err != nil {
		return "", time.Time{}, errcode.ErrMinIODownloadFailed.WithDetail("获取下载链接失败: " + err.Error())
	}

	return presignedURL.String(), time.Now().Add(expiry), nil
}

// GetFile 获取文件内容
func (m *MinioClient) GetFile(ctx context.Context, objectKey string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, errcode.ErrMinIODownloadFailed.WithDetail("获取文件失败: " + err.Error())
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, errcode.ErrMinIODownloadFailed.WithDetail("读取文件失败: " + err.Error())
	}
	return data, nil
}


