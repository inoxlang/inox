package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/utils"
)

func CheckProgram(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	if len(mainSubCommandArgs) == 0 {
		fmt.Fprintf(errW, "missing script path\n")
		return ERROR_STATUS_CODE
	}

	fpath := mainSubCommandArgs[0]
	dir := getScriptDir(fpath)

	compilationCtx := createCompilationCtx(dir)
	inoxprocess.RestrictProcessAccess(compilationCtx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: false})

	data := inox_ns.GetCheckData(fpath, compilationCtx, outW)
	fmt.Fprintf(outW, "%s\n\r", utils.Must(json.Marshal(data)))

	return 0
}
