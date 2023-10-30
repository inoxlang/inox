//go:build !linux

package inoxprocess

import (
	"github.com/inoxlang/inox/internal/core"
)

func restrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission) {

}
