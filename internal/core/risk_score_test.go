package internal

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestComputeProgramRiskScore(t *testing.T) {

	t.Run("empty manifest, empty code", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		mod := utils.Must(ParseInMemoryModule("manifest {}", InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest := utils.Must(mod.EvalManifest(ManifestEvaluationConfig{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		}))

		assert.Equal(t, RiskScore(1), ComputeProgramRiskScore(mod, manifest))
	})

	t.Run("read /home/user/**/* permission", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		mod := utils.Must(ParseInMemoryModule(`
			manifest {
				permissions: {read: %/home/user/**/*}
			}
		`, InMemoryModuleParsingConfig{
			Name:    "",
			Context: ctx,
		}))

		manifest := utils.Must(mod.EvalManifest(ManifestEvaluationConfig{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		}))

		expected := RiskScore(FS_READ_PERM_RISK_SCORE * UNKNOW_FILE_PATTERN_SENSITIVITY_MUTLIPLIER)
		assert.Equal(t, expected, ComputeProgramRiskScore(mod, manifest))
	})

	t.Run("read /home/user/**/* permission & read https://** permission", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

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

		manifest := utils.Must(mod.EvalManifest(ManifestEvaluationConfig{
			GlobalConsts: mod.MainChunk.Node.GlobalConstantDeclarations,
		}))

		expected := RiskScore(
			(FS_READ_PERM_RISK_SCORE * UNKNOW_FILE_PATTERN_SENSITIVITY_MUTLIPLIER) *
				(HTTP_READ_PERM_RISK_SCORE * HOST_PATTERN_RISK_MULTIPLIER),
		)
		assert.Equal(t, expected, ComputeProgramRiskScore(mod, manifest))
	})
}
