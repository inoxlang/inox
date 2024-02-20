package permkind

import "github.com/inoxlang/inox/internal/utils"

var (
	PERMISSION_KINDS = []struct {
		PermissionKind PermissionKind
		Name           string
	}{
		{Read, "read"},
		{Write, "write"},
		{Delete, "delete"},
		{Use, "use"},
		{Consume, "consume"},
		{Provide, "provide"},
		{See, "see"},

		{Update, "update"},
		{Create, "create"},
		{WriteStream, "write-stream"},
	}

	PERMISSION_KIND_NAMES = utils.MapSlice(PERMISSION_KINDS, func(e struct {
		PermissionKind PermissionKind
		Name           string
	}) string {
		return e.Name
	})
)

/*
	read
	write/update
	write/write-stream
	write/append
*/

type PermissionKind int

const (
	//up to 16 major permission kinds
	Read    = PermissionKind(1 << iota)
	Write   = PermissionKind(1 << iota)
	Delete  = PermissionKind(1 << iota)
	Use     = PermissionKind(1 << iota)
	Consume = PermissionKind(1 << iota)
	Provide = PermissionKind(1 << iota)
	See     = PermissionKind(1 << iota)

	//up to 16 minor permission kinds for each major one
	Update      = Write + (1 << 16)
	Create      = Write + (2 << 16)
	WriteStream = Write + (4 << 16)
)

func (k PermissionKind) Major() PermissionKind {
	return k & 0xffff
}

func (k PermissionKind) IsMajor() bool {
	return k == (k & 0xffff)
}

func (k PermissionKind) IsMinor() bool {
	return !k.IsMajor()
}

func (k PermissionKind) Includes(otherKind PermissionKind) bool {
	return k.Major() == otherKind.Major() && ((k.IsMajor() && otherKind.IsMinor()) || k == otherKind)
}

func (kind PermissionKind) String() string {
	if kind < 0 {
		return "<invalid permission kind>"
	}

	for _, e := range PERMISSION_KINDS {
		if e.PermissionKind == kind {
			return e.Name
		}
	}

	return "<invalid permission kind>"
}

func PermissionKindFromString(s string) (PermissionKind, bool) {
	for _, e := range PERMISSION_KINDS {
		if e.Name == s {
			return e.PermissionKind, true
		}
	}

	return 0, false
}

func IsPermissionKindName(s string) bool {
	_, ok := PermissionKindFromString(s)
	return ok
}
