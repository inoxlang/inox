package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSharable(t *testing.T) {

	t.Run("object", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		state := ctx.GetClosestState()

		assert.True(t, utils.Ret0(NewObjectFromMap(ValMap{}, ctx).IsSharable(state)))
		assert.True(t, utils.Ret0(NewObjectFromMap(ValMap{"a": Int(1)}, ctx).IsSharable(state)))
		assert.True(t, utils.Ret0(NewObjectFromMap(ValMap{"a": NewWrappedValueList()}, ctx).IsSharable(state)))
		assert.True(t, utils.Ret0(NewObjectFromMap(ValMap{"a": NewWrappedValueList(Int(1))}, ctx).IsSharable(state)))

		inner := NewWrappedValueList(NewObjectFromMap(ValMap{"a": Int(1)}, ctx))
		assert.True(t, utils.Ret0(NewObjectFromMap(ValMap{"a": inner}, ctx).IsSharable(state)))
	})

	//TODO: add tests of ValueHistory & Mapping
}
