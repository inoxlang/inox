package css

import (
	"github.com/inoxlang/inox/internal/cache"
)

type StylesheetCache = cache.ParseCache[Node]

func NewParseCache() *StylesheetCache {
	return cache.NewParseCache[Node]()
}
