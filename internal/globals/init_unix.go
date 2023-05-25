//go:build unix

package internal

import (
	"os"

	core "github.com/inoxlang/inox/internal/core"
)

func targetSpecificInit() {
	core.SetInitialWorkingDir(os.Getwd)
}
