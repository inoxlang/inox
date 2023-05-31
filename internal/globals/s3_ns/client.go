package s3_ns

import (
	"github.com/minio/minio-go/v7"
)

type S3Client struct {
	libClient *minio.Client
}
