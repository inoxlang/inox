package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/utils"
)

func RunProgram(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	//read and check arguments

	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var useTreeWalking bool
	var enableTestingMode bool
	var enableTestingModeAndTrust bool
	var disableOptimization bool
	var fullyTrusted bool
	var allowBrowserAutomation bool

	flags.BoolVar(&enableTestingMode, "test", false, "enable testing mode")
	flags.BoolVar(&enableTestingModeAndTrust, "test-trusted", false, "enable testing mode and do not show confirmation prompt if the risk score is high")
	flags.BoolVar(&useTreeWalking, "t", false, "use tree walking interpreter")
	flags.BoolVar(&disableOptimization, "no-optimization", false, "disable bytecode optimization")
	flags.BoolVar(&fullyTrusted, "fully-trusted", false, "do not show confirmation prompt if the risk score is high")
	flags.BoolVar(&allowBrowserAutomation, "allow-browser-automation", false, "allow creating and controlling a browser")

	fileArgIndex := -1

	for i, arg := range mainSubCommandArgs {
		if arg != "" && arg[0] != '-' {
			fileArgIndex = i
			break
		}
	}

	if fileArgIndex == -1 { //file not found
		if slices.Contains(mainSubCommandArgs, "-h") {
			showHelp(flags, mainSubCommandArgs, outW)
			return
		}
		fmt.Fprintf(errW, "missing script path\n")
		showHelp(flags, mainSubCommandArgs, outW)
		return ERROR_STATUS_CODE
	}

	moduleArgs := mainSubCommandArgs[fileArgIndex+1:]
	mainSubCommandArgs = mainSubCommandArgs[:fileArgIndex+1]

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(outW, err)
		return
	}

	fpath := flags.Arg(0)

	if fpath == "" {
		fmt.Fprintf(errW, "missing script path\n")
		showHelp(flags, mainSubCommandArgs, outW)
		return ERROR_STATUS_CODE
	}

	if enableTestingModeAndTrust {
		fullyTrusted = true
		enableTestingMode = true
	}

	//create a temporary directory for the whole process
	_, processTempDirPerms, removeTempDir := CreateTempDir()
	defer removeTempDir()

	//Initializations.

	tailwind.InitSubset()

	//run script

	dir := getScriptDir(fpath)
	compilationCtx := createCompilationCtx(dir)

	compilationCtx.SetWaitConfirmPrompt(func(msg string, accepted []string) (bool, error) {
		if fullyTrusted {
			return true, nil
		}

		fmt.Fprint(outW, msg)
		var input string
		_, err := fmt.Scanln(&input)

		if err != nil && err.Error() == "unexpected newline" {
			return false, nil
		}

		if err != nil {
			return false, err
		}
		input = strings.ToLower(input)
		return utils.SliceContains(accepted, input), nil
	})

	var testFilters core.TestFilters
	if enableTestingMode {
		testFilters = core.TestFilters{
			PositiveTestFilters: []core.TestFilter{
				{
					NameRegex: ".*",
				},
			},
		}
	}

	if allowBrowserAutomation {
		chrome_ns.AllowBrowserAutomation()
	}

	res, scriptState, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
		Fpath:                     fpath,
		PassedCLIArgs:             moduleArgs,
		PreinitFilesystem:         compilationCtx.GetFileSystem(),
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil, //grant all permissions
		ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		AdditionalPermissions:     processTempDirPerms,

		Transpile: !useTreeWalking,
		Out:       outW,

		FullAccessToDatabases: true,
		EnableTesting:         enableTestingMode,
		TestFilters:           testFilters,

		OnPrepared: func(state *core.GlobalState) error {
			inoxprocess.RestrictProcessAccess(state.Ctx, inoxprocess.ProcessRestrictionConfig{
				AllowBrowserAccess: true,
				BrowserBinPath:     chrome_ns.BROWSER_BINPATH,
			})
			return nil
		},
	})

	prettyPrintConfig := config.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(compilationCtx) // TODO: use another context?

	if err != nil {
		var assertionErr *core.AssertionError
		var errString string

		if errors.As(err, &assertionErr) {
			errString = assertionErr.PrettySPrint(prettyPrintConfig)

			if !assertionErr.IsTestAssertion() {
				errString += "\n" + utils.StripANSISequences(err.Error())
			}
		} else {
			errString = utils.StripANSISequences(err.Error())
		}

		//print
		errString = utils.AddCarriageReturnAfterNewlines(errString)
		fmt.Fprint(errW, errString, "\r\n")

		if errors.Is(err, chrome_ns.ErrBrowserAutomationNotAllowed) {
			fmt.Fprintf(errW, "did you forget to add the --allow-browser-automation switch ?\r\n")
		}

	} else {
		if list, ok := res.(*core.List); (!ok && res != nil) || (ok && list.Len() != 0) {
			core.PrettyPrint(res, outW, prettyPrintConfig, 0, 0)
			outW.Write([]byte("\r\n"))
		}
	}

	//print test suite results

	if scriptState == nil || len(scriptState.TestingState.SuiteResults) == 0 {
		return
	}

	outW.Write(utils.StringAsBytes("TEST RESULTS\n\r\n\r"))

	colorized := config.DEFAULT_PRETTY_PRINT_CONFIG.Colorize
	backgroundIsDark := config.INITIAL_BG_COLOR.IsDarkBackgroundColor()

	for _, suiteResult := range scriptState.TestingState.SuiteResults {
		msg := utils.AddCarriageReturnAfterNewlines(suiteResult.MostAdaptedMessage(colorized, backgroundIsDark))
		fmt.Fprint(outW, msg)
	}

	return 0
}
