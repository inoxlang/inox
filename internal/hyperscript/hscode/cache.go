package hscode

import (
	"github.com/inoxlang/inox/internal/cache/parsecache"
)

type FileParseCache = parsecache.Cache[ParsingResult, error]

func NewParseCache() *FileParseCache {
	return parsecache.New[ParsingResult, error]()
}
