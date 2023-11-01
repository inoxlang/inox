//go:build unix

package internal

import (
	"os"

	"github.com/inoxlang/inox/internal/core"
)

func targetSpecificInit() {
	core.SetInitialWorkingDir(os.Getwd)
}
