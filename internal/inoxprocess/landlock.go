package inoxprocess

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/shoenig/go-landlock"
)

type ProcessRestrictionConfig struct {
	AllowBrowserAccess bool
	BrowserBinPath     string
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

	if config.AllowBrowserAccess && config.BrowserBinPath != "" {
		additionalPaths = append(additionalPaths, landlock.File(
			config.BrowserBinPath,
			"rx",
		))
	}

	if config.ForceAllowDNS {
		additionalPaths = append(additionalPaths, landlock.DNS())
	}

	restrictProcessAccess(grantedPerms, forbiddenPerms, fls, additionalPaths)
}
