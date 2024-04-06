package permbase

import (
	"errors"
)

var (
	ErrImpossibleToVerifyPermissionForUrlHolderMutation = errors.New("impossible to verify permission for mutation of URL holder")
)

// A Permission carries a kind and can include narrower permissions, for example
// (read http://**) includes (read https://example.com).
type Permission interface {
	Kind() PermissionKind
	InternalPermTypename() InternalPermissionTypename
	Includes(Permission) bool
	String() string
}
