package net_ns

import (
	"errors"
)

const (
	COOKIE_KV_KEY = "__cookies"
)

var (
	ErrInvalidPersistedCookies = errors.New("invalid persisted cookies")
)
