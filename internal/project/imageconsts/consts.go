package imageconsts

import "github.com/inoxlang/inox/internal/core"

var (
	//Source control.
	ABSOLUTE_EXCLUSION_FILTERS = []core.PathPattern{
		"/**/.*",   //files whose name starts with a dot
		"/**/.*/",  //directories whose name starts with a dot
		"/**/.*/*", //files in directories whose name starts with a dot
	}

	RELATIVE_EXCLUSION_FILTERS = []core.PathPattern{
		"**/.*",   //files whose name starts with a dot
		"**/.*/",  //directories whose name starts with a dot
		"**/.*/*", //files in directories whose name starts with a dot
	}
)
