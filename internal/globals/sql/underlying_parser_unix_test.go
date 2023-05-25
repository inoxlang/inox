package internal

import (
	"testing"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestSQLQueryParser(t *testing.T) {

	t.Run("validate", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			parser := newQueryParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, ""))
		})

		t.Run("space", func(t *testing.T) {
			parser := newQueryParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, " "))
		})

		t.Run("valid", func(t *testing.T) {
			parser := newQueryParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.True(t, parser.Validate(ctx, "select * from users;"))
		})

		t.Run("two valid statements", func(t *testing.T) {
			parser := newQueryParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, "select * from users; select * from users;"))
		})

		t.Run("three valid statements", func(t *testing.T) {
			parser := newQueryParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, "select * from users; select * from users; select * from users;"))
		})
	})

}

func TestSQLStringParser(t *testing.T) {

	t.Run("validate", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, ""))
		})

		t.Run("space", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, " "))
		})

		t.Run("emtpy single-quoted string literal", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.True(t, parser.Validate(ctx, "''"))
		})

		t.Run("single-char single-quoted string literal", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.True(t, parser.Validate(ctx, "'a'"))
		})

		t.Run("emtpy double-quoted string literal", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.True(t, parser.Validate(ctx, `""`))
		})

		t.Run("single-char double-quoted string literal", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.True(t, parser.Validate(ctx, `"a"`))
		})

		t.Run("binary operation starting with a string", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, "'' or 1=1"))
		})

		t.Run("binary operation finishing with a string", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, "1+1 or ''"))
		})

		t.Run("unterminated empty single-quoted string", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, "'"))
		})

		t.Run("unterminated single-char single-quoted string", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, `'a`))
		})

		t.Run("unterminated empty double-quoted string", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, `"`))
		})

		t.Run("unterminated single-char double-quoted string", func(t *testing.T) {
			parser := newStringValueParser()
			ctx := core.NewContext(core.ContextConfig{})
			assert.False(t, parser.Validate(ctx, `"a`))
		})
	})
}
