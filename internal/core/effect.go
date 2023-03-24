package internal

import "errors"

var (
	ErrEffectAlreadyApplied = errors.New("effect is already applied")
	ErrIrreversible         = errors.New("effect is irreversible")
)

type Effect interface {
	Resources() []ResourceName
	PermissionKind() PermissionKind
	Reversability(*Context) Reversability
	IsApplied() bool
	Apply(*Context) error
	Reverse(*Context) error
}

type Reversability int

const (
	Irreversible Reversability = iota
	SomewhatReversible
	Reversible
)
