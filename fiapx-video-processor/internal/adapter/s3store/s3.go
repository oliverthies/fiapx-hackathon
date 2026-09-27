package s3store

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
)

var _ application.Storage = (*Storage)(nil)

type Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
	UseSSL    bool
}

type Storage struct {
	client *minio.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Storage, error) {
	endpoint, useSSL := parseEndpoint(cfg.Endpoint, cfg.UseSSL)
	if endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("S3_ENDPOINT and S3_BUCKET are required")
	}
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	}
	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, err
	}
	s := &Storage{client: client, bucket: cfg.Bucket}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Storage) SaveOriginal(ctx context.Context, jobID uuid.UUID, filename string, r io.Reader) (string, error) {
	key := path.Join("uploads", jobID.String()+filepath.Ext(filename))
	if _, err := s.Put(ctx, key, r); err != nil {
		return "", err
	}
	return key, nil
}

func (s *Storage) Put(ctx context.Context, key string, r io.Reader) (int64, error) {
	info, err := s.client.PutObject(ctx, s.bucket, key, r, -1, minio.PutObjectOptions{
		ContentType: contentType(key),
	})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, err
	}
	return obj, nil
}

func (s *Storage) Remove(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil && isNotFound(err) {
		return nil
	}
	return err
}

func parseEndpoint(raw string, useSSL bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "https://"):
		return strings.TrimPrefix(raw, "https://"), true
	case strings.HasPrefix(raw, "http://"):
		return strings.TrimPrefix(raw, "http://"), false
	default:
		return raw, useSSL
	}
}

func contentType(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".mp4":
		return "video/mp4"
	case ".mov", ".qt":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".zip":
		return "application/zip"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	if resp.StatusCode == http.StatusNotFound {
		return true
	}
	switch resp.Code {
	case "NoSuchKey", "NotFound", "NoSuchObject":
		return true
	default:
		return false
	}
}
