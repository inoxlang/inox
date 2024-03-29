package binary

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/inoxlang/inox/internal/utils/processutils"
	"github.com/rs/zerolog"
)

const (
	SRC_DIRNAME = "src" //directory containing the code in working directoryof the Deno process.
)

type Execution struct {
	Location string

	//Temporary directory the binary is allowed to access.
	//Writing to src/ is denied.
	AbsoluteWorkDir string

	//Cache directory. It should be used by a single process and should be different from the working directory.
	AbsoluteDenoDir string

	RelativeProgramPath string

	CLIArguments []string

	IgnoredCertificateErrors []string //host list

	AllowNetwork bool

	AllowLocalhostAccess bool

	Logger zerolog.Logger

	StartEventChan chan int32

	writeToStdoutStderr bool
}

func ExecuteWithAutoRestart(ctx context.Context, args Execution) error {

	if reflect.ValueOf(args.Logger).IsZero() {
		args.Logger = zerolog.Nop()
	}

	return processutils.AutoRestart(processutils.AutoRestartArgs{
		ProcessNameInLogs: "deno",
		GoCtx:             ctx,
		Logger:            args.Logger,
		StartEventChan:    args.StartEventChan,
		MakeCommand: func(goCtx context.Context) (*exec.Cmd, error) {
			return makeCommand(goCtx, args)
		},
	})
}

func makeCommand(ctx context.Context, args Execution) (*exec.Cmd, error) {

	//Check arguments.

	location, workDir, denoDir := args.Location, args.AbsoluteWorkDir, args.AbsoluteDenoDir

	workDir = filepath.Clean(workDir)
	denoDir = filepath.Clean(denoDir)
	srcDir := filepath.Join(workDir, SRC_DIRNAME)

	if !filepath.IsAbs(workDir) {
		return nil, fmt.Errorf("working dir should be absolute")
	}

	if !filepath.IsAbs(denoDir) {
		return nil, fmt.Errorf("DENO_DIR dir should be absolute")
	}

	if workDir == denoDir {
		return nil, fmt.Errorf("working dir should not be DENO_DIR")
	}

	//Check the Deno binary.

	stat, err := os.Stat(location)
	if err != nil {
		return nil, err
	}

	if stat.Mode().Perm() != WANTED_FILE_PERMISSIONS {
		return nil, fmt.Errorf("the Deno binary at %q does not have the wanted unix permissions (%s)", location, WANTED_FILE_PERMISSIONS.String())
	}

	//Create the command.

	commandArgs := []string{
		"run",
		//permissions
		`--allow-env`,
		`--allow-read=` + workDir + "," + denoDir,
		"--allow-write=" + workDir + "," + denoDir,
		`--deny-write=/etc,/var/run/,` + srcDir + "," + args.RelativeProgramPath,
		`--deny-read=/etc,/var/run/`,
	}

	if len(args.IgnoredCertificateErrors) > 0 {
		commandArgs = append(commandArgs, `--unsafely-ignore-certificate-errors=`+strings.Join(args.IgnoredCertificateErrors, ","))
	}

	if args.AllowNetwork {
		commandArgs = append(commandArgs, "--allow-net")
	}

	if !args.AllowLocalhostAccess {
		commandArgs = append(commandArgs, "--deny-net=localhost")
	}

	//program
	commandArgs = append(commandArgs, args.RelativeProgramPath)
	commandArgs = append(commandArgs, args.CLIArguments...)

	cmd := exec.CommandContext(ctx, location, commandArgs...)

	cmd.Dir = workDir

	args.Logger.Debug().Msg("command is " + cmd.String())

	if args.writeToStdoutStderr {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	cmd.Env = []string{
		"HOME=" + workDir,

		//https://docs.deno.com/runtime/manual/getting_started/setup_your_environment#environment-variables
		"NO_COLOR=true",
		"DENO_TLS_CA_STORE=mozilla",
		"DENO_NO_PACKAGE_JSON=true",
		"DENO_NO_UPDATE_CHECK=true",
		"DENO_NO_PROMPT=true",
		"DENO_DIR=" + denoDir,
	}

	return cmd, nil
}
