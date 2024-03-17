package analysis

import (
	"github.com/inoxlang/inox/internal/core"
)

// InoxModule contains information about an Inox module.
// All fields may be nil.
type InoxModule struct {
	PreparationError error
	Module           *core.Module

	StaticCheckData *core.StaticCheckData
	SymbolicData    *core.SymbolicData
	Manifest        *core.Manifest
}

func (a *analyzer) analyzeInoxModuleAndIncludedFiles() {

}
