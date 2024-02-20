package internal

import (
	"io"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestCheckHelpDataOnBuiltins(t *testing.T) {
	testconfig.AllowParallelization(t)

	help.ForeachTopicGroup(func(name string, group help.TopicGroup) {
		if !group.AboutBuiltins {
			return
		}

		t.Run(name, func(t *testing.T) {
			testconfig.AllowParallelization(t)

			fls := fs_ns.NewMemFilesystem(1_000)
			ctx := core.NewContextWithEmptyState(core.ContextConfig{
				Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			for _, topic := range group.Elements {
				t.Run(topic.Topic, func(t *testing.T) {
					checkTopic(t, topic, ctx)
					for _, subtopic := range topic.SubTopics {
						checkTopic(t, subtopic, ctx)
					}
				})
			}
		})
	})
}

func checkTopic(t *testing.T, topic help.TopicHelp, ctx *core.Context) {
	for _, example := range topic.Examples {
		t.Run(example.Code, func(t *testing.T) {
			fls := ctx.GetFileSystem()
			err := util.WriteFile(fls, "/main.ix", []byte("manifest {}\n"+example.Code), 0600)
			if !assert.NoError(t, err) {
				return
			}

			state, _, _, _ := core.PrepareLocalModule(core.ModulePreparationArgs{
				Fpath:                     "/main.ix",
				ParsingCompilationContext: ctx,
				DataExtractionMode:        true,
				Out:                       io.Discard,
				Logger:                    zerolog.Nop(),
				ScriptContextFileSystem:   fls,
			})

			if !assert.NotNil(t, state) {
				return
			}

			assert.Empty(t, state.Module.ParsingErrors)

			if example.Standalone {
				assert.Empty(t, state.StaticCheckData.Errors())
			}

			//TODO: make sure sure all examples can be prepared without typechecking errors.
			//assert.NoError(t, err)
		})
	}
}
