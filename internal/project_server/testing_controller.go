package project_server

import "sync"

type TestingController struct {
	lock sync.Mutex

	rootTestItems []TestItem
}

type TestItem struct {
	kind TestItemKind
	path string
}

type TestItemKind int8

const (
	ModuleTestItem TestItemKind = iota + 1
	DirTestItem
	TestSuite
	TestCase
)

// DoContinuousDiscovery discovers tests continuously by watching the filesystem for changes,
// DoContinuousDiscovery runs in the caller's goroutine.
func (c *TestingController) DoContinuousDiscovery() {

}
