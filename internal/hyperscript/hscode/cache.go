package hscode

import (
	"github.com/inoxlang/inox/internal/cache/parsecache"
)

type ParseCache = parsecache.Cache[ParsingResult, error /* *ParsingError or critical error */]

func NewParseCache() *ParseCache {
	return parsecache.New[ParsingResult, error]()
}
