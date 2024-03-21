package core

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core/golang/gen"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/inoxconsts"
)

// A TranspiledApp represents an Inox application transpiled to Golang, it does not hold any state and should NOT be modified.
type TranspiledApp struct {
	mainModuleName      ResourceName
	mainPkg             *gen.Pkg
	inoxModules         map[ResourceName]*TranspiledModule
	transpilationConfig AppTranspilationConfig
}

func (a *TranspiledApp) GetModule(resourceName ResourceName) (*TranspiledModule, bool) {
	mod, ok := a.inoxModules[resourceName]
	return mod, ok
}

// WriteToFilesystem writes the source code of the transpiled application to $ctx's filesystem at $srcDir.
// This also includes source code from Inox's codebase.
func (a *TranspiledApp) WriteToFilesystem(ctx *Context, srcDir string) error {

	fls := ctx.GetFileSystem()

	if fls == nil {
		return errors.New("context has no filesystem")
	}

	//Check that we are allowed to write to the filesystem.

	err := ctx.CheckHasPermission(FilesystemPermission{
		Kind_:  permkind.Write,
		Entity: DirPathFrom(srcDir).ToPrefixPattern(),
	})

	if err != nil {
		return err
	}

	//Write the Inox codebase.

	visitEmbeddedEntry := func(embeddedEntryPath string, d fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		path := fls.Join(srcDir, embeddedEntryPath)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if info.IsDir() {
			//Create the directory

			return fls.MkdirAll(path, 0700)
		}

		//Ignore non-regular files
		if !info.Mode().IsRegular() {
			return nil
		}

		//Create the file with the same content.
		content, err := fs.ReadFile(InoxCodebaseFS, embeddedEntryPath)
		if err != nil {
			return fmt.Errorf("failed to read %s from Inox's codebase: %w", embeddedEntryPath, err)
		}

		return util.WriteFile(fls, path, content, 0600)
	}

	for _, rootEntryName := range []string{"internal", inoxconsts.RELATIVE_MAIN_INOX_MOD_PKG_PATH, "go.mod", "go.sum"} {
		err = fs.WalkDir(InoxCodebaseFS, rootEntryName, visitEmbeddedEntry)
		if err != nil {
			return err
		}
	}

	//Write the transpiled modules (packages).

	for _, mod := range a.inoxModules {
		dir := filepath.Join(srcDir, mod.relativePkgPath)
		err := fls.MkdirAll(dir, 0700)

		if err == nil {
			err = mod.pkg.WriteTo(fls, dir)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// A TranspiledModule represents an Inox module transpiled to Golang, it does not hold any state and should NOT be modified.
type TranspiledModule struct {
	name            ResourceName
	sourceModule    *Module
	pkg             *gen.Pkg
	pkgID           string //example: github.com/inoxlang/inox/app/routes/index_ix
	relativePkgPath string //example: app/routes/index_ix
}

func (m *TranspiledModule) ModuleName() ResourceName {
	return m.name
}

// TranspiledModule returns the Go package that contains the transpiled module,
// the result should not be modified.
func (m *TranspiledModule) Pkg() *gen.Pkg {
	return m.pkg
}

func (m *TranspiledModule) PkgID() string {
	return m.pkgID
}

func (m *TranspiledModule) RelativePkgPath() string {
	return m.relativePkgPath
}
