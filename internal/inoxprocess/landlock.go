package inoxprocess

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

// RestrictProcessAccess uses Landlock to restrict what files the process has access to.
// The function does nothing on non-Linux systems.
// Landlock:
// - https://landlock.io/
// - https://www.man7.org/linux/man-pages/man7/landlock.7.html
func RestrictProcessAccess(ctx *core.Context) {

	fls, ok := ctx.GetFileSystem().(*fs_ns.OsFilesystem)
	if !ok {
		panic(errors.New("filesystem should be the OS filesystem"))
	}

	restrictProcessAccess(ctx.GetGrantedPermissions(), ctx.GetForbiddenPermissions(), fls)
}
