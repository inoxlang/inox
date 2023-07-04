package core

import (
	"testing"
)

func TestResourceGraph(t *testing.T) {
	t.Run("", func(t *testing.T) {
		g := NewResourceGraph()

		g.AddResource(Path("/main.ix"), "module")
		g.AddResource(Path("/lib.ix"), "module")
		g.AddEdge(Path("/main.ix"), Path("/lib.ix"), CHUNK_IMPORT_MOD_REL)
	})
}
