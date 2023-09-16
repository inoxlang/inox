package s3_ns

import (
	"context"
	"io"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/minio/minio-go/v7"
)

const (
	OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME = "object-storage/request"
)

type S3Client struct {
	libClient *minio.Client
}

func (c *S3Client) GetObject(ctx *core.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return core.DoIO2(ctx, func() (*minio.Object, error) {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	})
}

func (c *S3Client) PutObject(ctx *core.Context, bucketName, objectName string, reader io.Reader, objectSize int64,
	opts minio.PutObjectOptions) (minio.UploadInfo, error) {

	return core.DoIO2(ctx, func() (minio.UploadInfo, error) {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
	})
}

func (c *S3Client) PutObjectNoCtx(bucketName, objectName string, reader io.Reader, objectSize int64,
	opts minio.PutObjectOptions) (minio.UploadInfo, error) {

	return c.libClient.PutObject(context.Background(), bucketName, objectName, reader, objectSize, opts)
}

func (c *S3Client) CopyObject(ctx *core.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
	return core.DoIO2(ctx, func() (minio.UploadInfo, error) {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.CopyObject(ctx, dst, src)
	})
}

func (c *S3Client) ListObjectsLive(ctx *core.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {

	channel := core.DoIO(ctx, func() <-chan minio.ObjectInfo {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.ListObjects(ctx, bucketName, opts)
	})

	return channel
}

func (c *S3Client) ListObjects(ctx *core.Context, bucketName string, opts minio.ListObjectsOptions) ([]*ObjectInfo, error) {
	channel := c.ListObjectsLive(ctx, bucketName, opts)
	prefixSlashClount := strings.Count(opts.Prefix, "/")

	var objects []*ObjectInfo
	for obj := range channel {
		if strings.HasSuffix(obj.Key, "/") && strings.Count(obj.Key, "/") == prefixSlashClount {
			continue
		}
		objects = append(objects, &ObjectInfo{ObjectInfo: obj})
	}

	return objects, nil
}

func (c *S3Client) RemoveObject(ctx *core.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error {
	return core.DoIO(ctx, func() error {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.RemoveObject(ctx, bucketName, objectName, opts)
	})
}

func (c *S3Client) RemoveObjects(ctx *core.Context, bucketName string, objectChan <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	return core.DoIO(ctx, func() <-chan minio.RemoveObjectError {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.RemoveObjects(ctx, bucketName, objectChan, opts)
	})
}

func (c *S3Client) GetBucketPolicy(ctx *core.Context, bucketName string) (string, error) {
	return core.DoIO2(ctx, func() (string, error) {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.GetBucketPolicy(ctx, bucketName)
	})
}

func (c *S3Client) SetBucketPolicy(ctx *core.Context, bucketName, content string) error {
	return core.DoIO(ctx, func() error {
		ctx.Take(OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, 1)
		return c.libClient.SetBucketPolicy(ctx, bucketName, content)
	})
}
