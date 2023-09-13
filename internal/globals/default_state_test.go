package internal

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultContext(t *testing.T) {

	t.Run("OS filesystem should not be used as a fallback if .Filesystem is set", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1000)
		ctx, err := NewDefaultContext(default_state.DefaultContextConfig{
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, fls, ctx.GetFileSystem())
	})

	t.Run("OS filesystem should not be used as a fallback if .ParentContext is set & has no filesystem", func(t *testing.T) {
		ctx, err := NewDefaultContext(default_state.DefaultContextConfig{
			ParentContext: core.NewContexWithEmptyState(core.ContextConfig{}, nil),
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Nil(t, ctx.GetFileSystem())
	})

	t.Run("OS filesystem should not be used as a fallback if .ParentContext is set & has a filesystem", func(t *testing.T) {
		parentFls := fs_ns.NewMemFilesystem(1000)

		ctx, err := NewDefaultContext(default_state.DefaultContextConfig{
			ParentContext: core.NewContexWithEmptyState(core.ContextConfig{
				Filesystem: parentFls,
			}, nil),
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, parentFls, ctx.GetFileSystem())
	})

	t.Run("OS filesystem should not be used as a fallback if both .Filesystem & .ParentContext are set", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1000)
		ctx, err := NewDefaultContext(default_state.DefaultContextConfig{
			Filesystem:    fls,
			ParentContext: core.NewContexWithEmptyState(core.ContextConfig{}, nil),
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, fls, ctx.GetFileSystem())
	})

	t.Run("OS filesystem should be used as a fallback if both .Filesystem & .ParentContext are not set", func(t *testing.T) {
		ctx, err := NewDefaultContext(default_state.DefaultContextConfig{})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, fs_ns.GetOsFilesystem(), core.WithoutSecondaryContextIfPossible(ctx.GetFileSystem()))
	})
}
