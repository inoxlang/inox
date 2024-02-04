package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoerceToBool(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []struct {
		name  string
		input Value
		ok    bool
	}{
		{"nil", Nil, false},

		{"true", True, true},
		{"false", False, false},

		{"empty string", String(""), false},
		{"non-empty string", String("1"), true},

		{"empty byte slice", NewMutableByteSlice([]byte{}, ""), false},
		{"non-empty byte slice", NewMutableByteSlice([]byte{'a'}, ""), true},

		{"string concatenation", NewStringConcatenation(String("a"), String("b")), true},

		{"bytes concatenation", NewBytesConcatenation(NewMutableByteSlice([]byte{'a'}, ""), NewMutableByteSlice([]byte{'b'}, "")), true},

		{"zero integral", Int(0), false},
		{"non-zero integral", Int(1), true},

		{"zero rate", ByteRate(0), false},
		{"non-zero rate", ByteRate(1), true},

		{"empty indexable (list)", NewWrappedValueList(), false},
		{"non-empty indxable (list)", NewWrappedValueList(Int(1)), true},

		{"empty container (object)", NewObject(), false},

		{"empty key list", (KeyList)(nil), false},
		{"non-empty key list", (KeyList)(nil), false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.True(t, testCase.ok == coerceToBool(ctx, testCase.input))
		})
	}
}
