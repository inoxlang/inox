package inoxmod

import "sync"

var (
	moduleCache     = map[string]string{}
	moduleCacheLock sync.Mutex
)

func UpdateModuleImportCache(hash string, content string) {
	moduleCache[hash] = content
}
