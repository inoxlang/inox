package globals

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globalnames"
)

var (
	GLOBAL_FUNCTIONS = map[string]core.Value{
		// events
		globalnames.EVENT_SRC_FN: core.ValOf(core.NewEventSource),

		//watch
		globalnames.VALUE_HISTORY_FN: core.WrapGoFunction(core.NewValueHistory),

		//conversion
		globalnames.TOJSON_FN:  core.ValOf(core.ToJSON),
		globalnames.TOPJSON_FN: core.ValOf(core.ToPrettyJSON),
		globalnames.ASJSON_FN:  core.ValOf(core.AsJSON),
		globalnames.ASJSONL_FN: core.ValOf(core.AsJSONL),

		//time
		globalnames.SLEEP_FN: core.ValOf(core.Sleep),

		//functional
		globalnames.MAP_ITERABLE_FN: core.WrapGoFunction(core.MapIterable),

		//other
		globalnames.FILEMODE_FN: core.WrapGoFunction(core.FileModeFrom),
	}
)
