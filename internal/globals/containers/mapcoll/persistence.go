package mapcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/jsoniter"
)

func persistMap(ctx *core.Context, map_ *Map, path core.Path, storage core.DataStore) error {
	//TODO

	panic(core.ErrNotImplementedYet)
}

func (m *Map) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	panic(core.ErrNotImplementedYet)
	w.WriteArrayStart()

	//TODO: implement

	w.WriteArrayEnd()
	return nil
}

func (m *Map) Migrate(ctx *core.Context, key core.Path, migration *core.FreeEntityMigrationArgs) (core.Value, error) {
	panic(core.ErrNotImplementedYet)
}
