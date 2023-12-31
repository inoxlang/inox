package core

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrCollectionElemNotFound = errors.New("collection element not found")
)

type Collection interface {
	Container

	//GetElementByKey should retrieve the element with the associated key.
	//ErrCollectionElemNotFound should be returned in the case of an error.
	//Implementation-specific errors are allowed.
	GetElementByKey(key ElementKey) (Serializable, error)
}

// An element key is a a string that:
// is at most 100-character long
// is not empty
// can only contain identifier chars (parse.IsIdentChar)
type ElementKey string

func ElementKeyFrom(key string) (ElementKey, error) {
	fmtErr := func(msg string) error {
		return fmt.Errorf("provided key %q is not a valid element key: %s", key, msg)
	}
	if len(key) == 0 {
		return "", fmtErr("empty")
	}

	if len(key) > 100 {
		return "", fmtErr("too long")
	}

	for _, r := range key {
		if !parse.IsIdentChar(r) {
			return "", fmtErr("invalid char found")
		}
	}
	return ElementKey(key), nil
}

func MustElementKeyFrom(key string) ElementKey {
	return utils.Must(ElementKeyFrom(key))
}
