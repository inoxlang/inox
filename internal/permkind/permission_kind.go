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
	Read    PermissionKind = (1 << iota)
	Write                  = (1 << iota)
	Delete                 = (1 << iota)
	Use                    = (1 << iota)
	Consume                = (1 << iota)
	Provide                = (1 << iota)
	See                    = (1 << iota)
	//up to 16 major permission kinds

	Update      = Write + (1 << 16)
	Create      = Write + (2 << 16)
	WriteStream = Write + (4 << 16)
	//up to 16 minor permission kinds for each major one
)

func (k PermissionKind) Major() PermissionKind {
	return k & 0xff
}

func (k PermissionKind) IsMajor() bool {
	return k == (k & 0xff)
}

func (k PermissionKind) IsMinor() bool {
	return k != (k & 0xff)
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
