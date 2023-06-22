package project

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestOpenRegistry(t *testing.T) {

	t.Run("once", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

		fls := fs_ns.NewMemFilesystem(1_000)

		r, err := OpenRegistry("/projects", fls)
		assert.NoError(t, err)

		r.Close(ctx)
	})

	t.Run("twice", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		fls := fs_ns.NewMemFilesystem(1_000)

		r, err := OpenRegistry("/projects", fls)
		assert.NoError(t, err)

		r.Close(ctx)

		r, err = OpenRegistry("/projects", fls)
		assert.NoError(t, err)

		r.Close(ctx)
	})

}
