package core

import (
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func createTestLifetimeJob(t *testing.T, state *GlobalState, code string) *LifetimeJob {
	jobMod := &Module{
		ModuleKind: LifetimeJobModule,
		MainChunk: parse.NewParsedChunk(parse.MustParseChunk(code), parse.InMemorySource{
			NameString: "test",
			CodeString: code,
		}),
	}

	job, err := NewLifetimeJob(Identifier("job"), nil, jobMod, nil, state)
	if !assert.NoError(t, err) {
		return nil
	}
	return job
}
