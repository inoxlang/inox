package inoxprocess

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/shoenig/go-landlock"
)

type ProcessRestrictionConfig struct {
	AllowBrowserAccess bool
	ForceAllowDNS      bool
}

// RestrictProcessAccess uses Landlock to restrict what files the process has access to.
// The function does nothing on non-Linux systems.
// Landlock:
// - https://landlock.io/
// - https://www.man7.org/linux/man-pages/man7/landlock.7.html
func RestrictProcessAccess(ctx *core.Context, config ProcessRestrictionConfig) {

	fls, ok := ctx.GetFileSystem().(*fs_ns.OsFilesystem)
	if !ok {
		panic(errors.New("filesystem should be the OS filesystem"))
	}

	grantedPerms := ctx.GetGrantedPermissions()
	forbiddenPerms := ctx.GetForbiddenPermissions()

	var additionalPaths []*landlock.Path

	if config.AllowBrowserAccess && chrome_ns.BROWSER_BINPATH != "" {
		additionalPaths = append(additionalPaths, landlock.File(
			chrome_ns.BROWSER_BINPATH,
			"rx",
		))
	}

	if config.ForceAllowDNS {
		additionalPaths = append(additionalPaths, landlock.DNS())
	}

	restrictProcessAccess(grantedPerms, forbiddenPerms, fls, additionalPaths)
}
