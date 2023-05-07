package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	help "github.com/inoxlang/inox/internal/globals/help"
)

func registerHelp() {
	help.RegisterHelpValues(map[string]any{
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

		//others
		"Error": _Error,
	})
}
