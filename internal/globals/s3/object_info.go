package internal

import (
	core "github.com/inox-project/inox/internal/core"
	"github.com/minio/minio-go/v7"
)

type ObjectInfo struct {
	core.NoReprMixin
	minio.ObjectInfo
}
