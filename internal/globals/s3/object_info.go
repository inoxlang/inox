package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/minio/minio-go/v7"
)

type ObjectInfo struct {
	core.NoReprMixin
	minio.ObjectInfo
}
