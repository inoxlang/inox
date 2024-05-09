package analysis

import (
	"github.com/inoxlang/inox/internal/core"
)

// InoxModuleInfo contains information about an Inox module.
// All fields may be nil.
type InoxModuleInfo struct {
	PreparationError error
	Module           *core.Module

	StaticCheckData *core.StaticCheckData
	SymbolicData    *core.SymbolicData
	Manifest        *core.Manifest

	state *core.GlobalState //temporary, removed when the result is returned.
}

func (a *analyzer) analyzeInoxModuleAndIncludedFiles() {

}
