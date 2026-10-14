package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// s3Client 是 S3 操作的窄接口，便于测试注入 mock（宪法 8.3：不过度抽象）。
type s3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

// S3Options 是构造 S3 存储所需的参数（storage 包不依赖 config，避免反向依赖）。
type S3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	ForcePathStyle  bool
}

// S3 实现 Storage 接口，直连模式下图片从对象存储/CDN 直接访问。
type S3 struct {
	client  s3Client
	bucket  string
	baseURL string
}

// NewS3 构造 S3 存储。path-style 默认开启以兼容 R2/MinIO/COS/OSS。
func NewS3(opts S3Options) (*S3, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(opts.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = opts.ForcePathStyle
		if opts.Endpoint != "" {
			o.BaseEndpoint = aws.String(opts.Endpoint)
		}
	})
	return &S3{client: client, bucket: opts.Bucket, baseURL: opts.PublicBaseURL}, nil
}

// Put 上传对象，并写入正确 Content-Type（直链可直显的关键）。
func (s *S3) Put(ctx context.Context, key string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	contentType := http.DetectContentType(data)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

// Get 拉取对象流。对象不存在时返回 os.ErrNotExist（与 Local 语义一致）。
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundErr(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	return out.Body, nil
}

// Delete 删除对象。对象不存在时不报错（幂等）。
func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// URL 返回绝对直链（PUBLIC_BASE_URL/key），见 Spec 第 3 节。
func (s *S3) URL(key string) string {
	return s.baseURL + "/" + key
}

// Head 返回对象大小；对象不存在时返回 os.ErrNotExist（migrate --verify 用）。
func (s *S3) Head(ctx context.Context, key string) (int64, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundErr(err) {
			return 0, os.ErrNotExist
		}
		return 0, err
	}
	return aws.ToInt64(out.ContentLength), nil
}

// Exists 判断对象是否已存在（migrate 幂等检查用）。
func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	if isNotFoundErr(err) {
		return false, nil
	}
	return false, err
}

// isNotFoundErr 识别 S3 各实现的对象不存在错误（NoSuchKey / NotFound / 404）。
// 用 smithy.APIError 的错误码统一判断，避免绑定具体错误类型。
func isNotFoundErr(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
