package inoxsh_ns

import (
	"testing"

	core "github.com/inoxlang/inox/internal/core"
	parse "github.com/inoxlang/inox/internal/parse"
	internal "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/stretchr/testify/assert"
)

func TestSprintPrompt(t *testing.T) {
	state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{
		Permissions: core.GetDefaultGlobalVarPermissions(),
	}))
	state.SetGlobal("whoami", core.ValOf(func(ctx *core.Context) core.Str {
		return "user"
	}), core.GlobalConst)

	printingConfig := PrintingConfig{
		prettyPrintConfig: &core.PrettyPrintConfig{
			PrettyPrintConfig: internal.PrettyPrintConfig{
				Colorize: false,
			},
		},
	}

	t.Run("no config", func(t *testing.T) {
		prompt, length := sprintPrompt(state, REPLConfiguration{
			PrintingConfig: printingConfig,
			prompt:         nil,
		})

		assert.Equal(t, "> ", prompt)
		assert.Equal(t, 2, length)
	})

	t.Run("empty list", func(t *testing.T) {
		prompt, length := sprintPrompt(state, REPLConfiguration{
			PrintingConfig: printingConfig,
			prompt:         core.NewWrappedValueList(),
		})

		assert.Equal(t, "", prompt)
		assert.Equal(t, 0, length)
	})

	t.Run("AST node", func(t *testing.T) {
		expr, _ := parse.ParseExpression("whoami()")
		prompt, length := sprintPrompt(state, REPLConfiguration{
			PrintingConfig: printingConfig,
			prompt:         core.NewWrappedValueList(core.AstNode{Node: expr}),
		})

		assert.Equal(t, "user", prompt)
		assert.Equal(t, 4, length)
	})

	t.Run("AST node with 2 colors", func(t *testing.T) {
		expr, _ := parse.ParseExpression("whoami()")
		prompt, length := sprintPrompt(state, REPLConfiguration{
			PrintingConfig: printingConfig,
			prompt: core.NewWrappedValueList(
				core.NewWrappedValueList(core.AstNode{Node: expr}, core.Identifier("white"), core.Identifier("black")),
			),
		})

		assert.Equal(t, "user", prompt)
		assert.Equal(t, 4, length)
	})

}
