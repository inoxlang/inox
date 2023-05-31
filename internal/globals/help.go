package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	help_ns "github.com/inoxlang/inox/internal/globals/help_ns"
)

func registerHelp() {
	help_ns.RegisterHelpValues(map[string]any{
		//functional
		"map":    core.Map,
		"filter": core.Filter,
		"some":   core.Some,
		"all":    core.All,
		"none":   core.None,
		"rand":   _rand,
		"find":   _find,
		"sort":   core.Sort,

		//resource manipulation
		"create": _createResource,
		"read":   _readResource,
		"update": _updateResource,
		"delete": _deleteResource,

		//encoding
		"b64":  encodeBase64,
		"db64": decodeBase64,

		"hex":   encodeHex,
		"unhex": decodeHex,

		//others
		"Error": _Error,
	})
}
