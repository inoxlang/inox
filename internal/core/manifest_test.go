package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/stretchr/testify/assert"
)

func TestModuleParameters(t *testing.T) {
	testconfig.AllowParallelization(t)

	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("one non positional parameter with no default value", func(t *testing.T) {
		params := &ModuleParameters{
			hasParamsRequiredOnCLI: true,
			others: []ModuleParameter{
				{
					name:                   "a",
					singleLetterCliArgName: 'a',
					cliArgName:             "-a",
					pattern:                PATH_PATTERN,
				},
			},
			paramsPattern: NewModuleParamsPattern([]string{"a"}, []Pattern{PATH_PATTERN}),
		}

		//No argument provided.
		args, err := params.GetArgumentsFromCliArgs(ctx, []string{})
		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, args)

		//One valid argument provided.
		args, err = params.GetArgumentsFromCliArgs(ctx, []string{"-a=/"})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, map[string]Value{"a": Path("/")}, args.ValueMap())
	})

}
