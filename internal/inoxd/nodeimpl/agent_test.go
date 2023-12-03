package nodeimpl

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestNewAgent(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		_, err := NewAgent(AgentParameters{
			GoCtx: ctx,
			Config: AgentConfig{
				OsProdDir:                       core.DirPathFrom(tmpDir),
				TemporaryOptionRunInSameProcess: true,
			},
		})

		assert.NoError(t, err)
	})

	t.Run("create: non-existing prod dir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		_, err := NewAgent(AgentParameters{
			GoCtx: ctx,
			Config: AgentConfig{
				OsProdDir: core.DirPathFrom("/tmp/non-existing-" + strconv.Itoa(rand.Int())),
			},
		})

		assert.ErrorContains(t, err, "failed to read entries of the prod directory")
	})

	t.Run("get/create application", func(t *testing.T) {
		tmpDir := t.TempDir()
		const APP_NAME = "myapp"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		agent, err := NewAgent(AgentParameters{
			GoCtx: ctx,
			Config: AgentConfig{
				OsProdDir:                       core.DirPathFrom(tmpDir),
				TemporaryOptionRunInSameProcess: true,
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		app, err := agent.GetOrCreateApplication(APP_NAME)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, app) {
			return
		}

		existing, err := agent.GetOrCreateApplication(APP_NAME)
		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, app, existing)
	})
}
