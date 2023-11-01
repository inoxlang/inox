package internal

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/inoxlang/inox/internal/core"
)

func _rand(ctx *core.Context, v core.Value, options ...core.Option) core.Value {
	if core.GetRandomnessSource(nil, options...) == nil {
		options = append(options, core.Option{Name: "source", Value: core.DefaultRandSource})
	}

	switch val := v.(type) {
	case core.Pattern:
		return val.Random(ctx, options...)
	case core.Indexable:
		if val.Len() == 0 {
			panic(fmt.Errorf("rand: cannot pick random element of empty indexable"))
		}
		maxIndex := big.NewInt(int64(val.Len()))
		n, _ := rand.Int(rand.Reader, maxIndex)
		return val.At(ctx, int(n.Int64()))
	default:
		panic(fmt.Errorf("rand: cannot generate random value from argument of type %T", v))
	}
}
