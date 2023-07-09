package s3_ns

import (
	"github.com/minio/minio-go/v7"
)

type ObjectInfo struct {
	minio.ObjectInfo
}
