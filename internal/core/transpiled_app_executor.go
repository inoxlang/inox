package core

import (
	"testing"
)

type TranspiledAppExecutor interface {
	App() *TranspiledApp
	CreateInstance(t *testing.T, ctx *Context) (TranspiledAppInstance, error)
}

type TranspiledAppInstance interface {
	IsRunning() bool
	Context() *Context
}
