//go:build unix

package globals

import (
	"os"

	"github.com/inoxlang/inox/internal/core"
)

func targetSpecificInit() {
	core.SetInitialWorkingDir(os.Getwd)
}
