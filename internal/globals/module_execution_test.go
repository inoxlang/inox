package internal

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	core "github.com/inox-project/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestRuneLocalScript(t *testing.T) {

	t.Run("a script with static check errors should not be runned", func(t *testing.T) {

		ctx := core.NewContext(core.ContextConfig{})
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		os.WriteFile(file, []byte("fn(){self}; return 1"), 0o600)

		res, _, _, err := RunLocalScript(RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: ctx,
			UseContextAsParent:        true,
			ParentContext:             ctx,
			Out:                       io.Discard,
		})

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}
