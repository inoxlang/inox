package inoxmod

import "maps"

type ModuleSet struct {
	modules map[ /*absolute source name*/ string]*Module
}

func NewModuleSet(modules map[string]*Module) *ModuleSet {
	return &ModuleSet{
		modules: maps.Clone(modules),
	}
}
