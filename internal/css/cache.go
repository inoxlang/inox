package css

import (
	"github.com/inoxlang/inox/internal/cache/parsecache"
)

type StylesheetCache = parsecache.Cache[Node, error]

func NewParseCache() *StylesheetCache {
	return parsecache.New[Node, error]()
}
