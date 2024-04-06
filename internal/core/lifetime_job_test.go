package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func createTestLifetimeJob(t *testing.T, state *GlobalState, code string) *LifetimeJob {
	chunk := parse.NewParsedChunkSource(parse.MustParseChunk(code), parse.InMemorySource{
		NameString: "test",
		CodeString: code,
	})

	jobMod := WrapLowerModule(&inoxmod.Module{
		Kind:         LifetimeJobModule,
		TopLevelNode: chunk.Node,
		MainChunk:    chunk,
	})

	job, err := NewLifetimeJob(Identifier("job"), nil, jobMod, state)
	if !assert.NoError(t, err) {
		return nil
	}
	return job
}
