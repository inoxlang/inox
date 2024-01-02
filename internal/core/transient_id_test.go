package core

import (
	"testing"
)

func BenchmarkTransientIdOf(b *testing.B) {

	ctx := NewDefaultTestContext()
	defer ctx.CancelGracefully()

	b.Run("registered core type", func(b *testing.B) {

		_, hasId := TransientIdOf(Int(1))
		if !hasId {
			b.Fatal("no id")
		}

		for i := 0; i < b.N; i++ {
			var value Value = Int(i)
			TransientIdOf(value)
		}
	})
}
