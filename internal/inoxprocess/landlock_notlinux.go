//go:build !linux

package inoxprocess

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/shoenig/go-landlock"
)

func restrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission, fls *fs_ns.OsFilesystem, additionalPaths []*landlock.Path) {

}
