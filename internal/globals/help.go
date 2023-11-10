package internal

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/help"
)

func registerHelp() {
	help.RegisterHelpValues(map[string]any{
		//functional
		globalnames.MAP_FN:         core.Map,
		globalnames.FILTER_FN:      core.Filter,
		globalnames.GET_AT_MOST_FN: core.GetAtMost,
		globalnames.SOME_FN:        core.Some,
		globalnames.ALL_FN:         core.All,
		globalnames.NONE_FN:        core.None,
		globalnames.RAND_FN:        _rand,
		globalnames.FIND_FN:        _find,
		globalnames.SORT_FN:        core.Sort,

		//resource manipulation
		globalnames.CREATE_FN: _createResource,
		globalnames.READ_FN:   _readResource,
		globalnames.UPDATE_FN: _updateResource,
		globalnames.DELETE_FN: _deleteResource,

		//encoding
		globalnames.B64_FN:  encodeBase64,
		globalnames.DB64_FN: decodeBase64,

		globalnames.HEX_FN:   encodeHex,
		globalnames.UNHEX_FN: decodeHex,

		//others
		globalnames.ERROR_FN: _Error,
	})
}
