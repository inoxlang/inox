package binary

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

var (
	DENO_BINARY_USED_FOR_EXEC_TESTS = "/tmp/deno-test"
)

func TestExecute(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	binaryLocation := DENO_BINARY_USED_FOR_EXEC_TESTS

	err := Install(binaryLocation)

	if !assert.NoError(t, err) {
		return
	}

	t.Run("base case", func(t *testing.T) {
		workDir := t.TempDir()
		programPath := filepath.Join(workDir, "index.js")
		denoDir := filepath.Join(workDir, ".cache")

		//Write a Deno program that creates a file and then enters an infinite loop.
		utils.PanicIfErr(
			os.WriteFile(programPath, []byte(`
				Deno.writeTextFileSync("a.txt", "hello", {create: true})
				while(true);
			`), 0500),
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			time.Sleep(200 * time.Millisecond)
			cancel()
		}()

		err := ExecuteWithAutoRestart(ctx, BinaryExecution{
			Location:            binaryLocation,
			AbsoluteWorkDir:     workDir,
			AbsoluteDenoDir:     denoDir,
			RelativeProgramPath: "index.js",
			Logger:              zerolog.New(os.Stdout),
		})

		if !assert.ErrorIs(t, err, context.Canceled) {
			return
		}

		content, err := os.ReadFile(filepath.Join(workDir, "a.txt"))

		if !assert.NoError(t, err, "the program should have been executed") {
			return
		}

		assert.Equal(t, []byte("hello"), content)
	})

	t.Run("the program should not be able to modify the entry JS/TS file", func(t *testing.T) {
		workDir := t.TempDir()
		programPath := filepath.Join(workDir, "index.js")
		denoDir := filepath.Join(workDir, ".cache")

		//Write a Deno program that modifies itself and then enters an infinite loop.
		program := `
			Deno.writeTextFileSync("before-program-update.txt", "", {create: true})
			Deno.writeTextFileSync("` + programPath + `", "hello", {create: true})
			Deno.writeTextFileSync("after-program-update.txt", "", {create: true})
			while(true);
		`

		utils.PanicIfErr(
			os.WriteFile(programPath, []byte(program), 0600),
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			time.Sleep(200 * time.Millisecond)
			cancel()
		}()

		err := ExecuteWithAutoRestart(ctx, BinaryExecution{
			Location:            binaryLocation,
			AbsoluteWorkDir:     workDir,
			AbsoluteDenoDir:     denoDir,
			RelativeProgramPath: "index.js",
			Logger:              zerolog.New(os.Stdout),
			writeToStdoutStderr: true,
		})

		if !assert.ErrorIs(t, err, context.Canceled) {
			return
		}

		//Check that the program has executed at least once.

		assert.FileExists(t, filepath.Join(workDir, "before-program-update.txt"))

		//Check that the program has not been changed.
		content, err := os.ReadFile(programPath)

		if !assert.NoError(t, err, "the program should have been executed") {
			return
		}

		assert.Equal(t, []byte(program), content)

		assert.NoFileExists(t, filepath.Join(workDir, "after-program-update.txt"))
	})
}
