package core

import (
	"testing"

	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestComputeProgramRiskScore(t *testing.T) {
	t.Run("empty manifest, empty code", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule("manifest {}", InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest, _, _, _ := mod.PreInit(PreinitArgs{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		})

		assert.Equal(t, RiskScore(1), utils.Ret0(ComputeProgramRiskScore(mod, manifest)))
	})

	t.Run("read /home/user/**/* permission", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {read: %/home/user/**/*}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest, _, _, _ := mod.PreInit(PreinitArgs{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		})

		expected := RiskScore(FS_READ_PERM_RISK_SCORE * UNKNOW_FILE_PATTERN_SENSITIVITY_MUTLIPLIER)
		assert.Equal(t, expected, utils.Ret0(ComputeProgramRiskScore(mod, manifest)))
	})

	t.Run("read /home/user/**/* permission & read https://** permission", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {
					read: {%/home/user/**/*, %https://**}
				}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest, _, _, _ := mod.PreInit(PreinitArgs{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		})

		expected := RiskScore(
			(FS_READ_PERM_RISK_SCORE * UNKNOW_FILE_PATTERN_SENSITIVITY_MUTLIPLIER) *
				(HTTP_READ_PERM_RISK_SCORE * HOST_PATTERN_RISK_MULTIPLIER),
		)
		assert.Equal(t, expected, utils.Ret0(ComputeProgramRiskScore(mod, manifest)))
	})

	t.Run("https://example.com/ permission for HTTP & Websocket", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {
					read: {wss://example.com/, https://example.com/}
				}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest, _, _, _ := mod.PreInit(PreinitArgs{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		})

		expectedHttpScore := RiskScore((HTTP_READ_PERM_RISK_SCORE * URL_RISK_MULTIPLIER))
		expectedScore := 2 * expectedHttpScore
		assert.Equal(t, expectedScore, utils.Ret0(ComputeProgramRiskScore(mod, manifest)))
	})

	t.Run("any-entity read http permission", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		ComputeProgramRiskScore(mod, &Manifest{
			RequiredPermissions: []Permission{
				HttpPermission{Kind_: permkind.Read, AnyEntity: true},
			},
		})
	})

	t.Run("create threads permission", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {create: {threads: {}}}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest, _, _, _ := mod.PreInit(PreinitArgs{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		})

		expected := RiskScore(LTHREAD_PERM_RISK_SCORE)
		assert.Equal(t, expected, utils.Ret0(ComputeProgramRiskScore(mod, manifest)))
	})

	t.Run("many permissions", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {
					read: {%/home/user/**/*, %https://**}
					write: {%/home/user/**/*, %https://**}
					provide: %https://**
				}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest, _, _, _ := mod.PreInit(PreinitArgs{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		})

		assert.Equal(t, MAXIMUM_RISK_SCORE, utils.Ret0(ComputeProgramRiskScore(mod, manifest)))
	})
}
