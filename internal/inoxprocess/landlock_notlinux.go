//go:build !linux

package inoxprocess

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

func restrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission, fls *fs_ns.OsFilesystem) {

}
