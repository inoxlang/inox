package local_db_ns

import (
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

const (
	LDB_SCHEME = core.Scheme("ldb")
)

func init() {
	core.RegisterSymbolicGoFunction(openDatabase, func(ctx *symbolic.Context, r symbolic.ResourceName) (*SymbolicLocalDatabase, *symbolic.Error) {
		return &SymbolicLocalDatabase{}, nil
	})

	core.RegisterOpenDbFn(LDB_SCHEME, func(ctx *core.Context, config core.DbOpenConfiguration) (core.Database, error) {
		return openDatabase(ctx, config.Resource, !config.FullAccess)
	})

	core.RegisterStaticallyCheckDbResolutionDataFn(LDB_SCHEME, func(node parse.Node) string {
		pathLit, ok := node.(*parse.AbsolutePathLiteral)
		if !ok || !strings.HasSuffix(pathLit.Value, "/") {
			return "the resolution data of a local database should be an absolute directory path (it should end with '/')"
		}

		return ""
	})
}

func NewLocalDbNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		//
		//"open": core.ValOf(openDatabase),
	})
}
