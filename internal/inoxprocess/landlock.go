package inoxprocess

import (
	"github.com/inoxlang/inox/internal/core"
)

// RestrictProcessAccess uses Landlock to restrict what files the process has access to.
// The function does nothing on non-Linux systems.
// Landlock:
// - https://landlock.io/
// - https://www.man7.org/linux/man-pages/man7/landlock.7.html
func RestrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission) {
	restrictProcessAccess(grantedPerms, forbiddenPerms)
}
