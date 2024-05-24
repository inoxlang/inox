package core_test

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
)

func BenchmarkTransientIdOf(b *testing.B) {

	ctx := NewDefaultTestContext()
	defer ctx.CancelGracefully()

	b.Run("registered core type", func(b *testing.B) {

		_, hasId := core.TransientIdOf(core.Int(1))
		if !hasId {
			b.Fatal("no id")
		}

		for i := 0; i < b.N; i++ {
			var value core.Value = core.Int(i)
			core.TransientIdOf(value)
		}
	})
}
