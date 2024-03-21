package core

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core/golang/gen"
	"github.com/inoxlang/inox/internal/core/permkind"
)

// A TranspiledApp represents an Inox application transpiled to Golang, it does not hold any state and should NOT be modified.
type TranspiledApp struct {
	mainModuleName      ResourceName
	mainPkg             *gen.Pkg
	inoxModules         map[ResourceName]*TranspiledModule
	transpilationConfig AppTranspilationConfig
}

func (a *TranspiledApp) WriteTo(ctx *Context, rootDir string) error {

	fls := ctx.GetFileSystem()

	if fls == nil {
		return errors.New("context has no filesystem")
	}

	//Check we are allowed to write to the filesystem.

	err := ctx.CheckHasPermission(FilesystemPermission{
		Kind_:  permkind.Write,
		Entity: DirPathFrom(rootDir).ToPrefixPattern(),
	})

	if err != nil {
		return err
	}

	//Write the Inox codebase.

	visitEmbeddedEntry := func(embeddedEntryPath string, d fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		path := fls.Join(rootDir, embeddedEntryPath)

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

	fs.WalkDir(InoxCodebaseFS, ".", visitEmbeddedEntry)

	// //Write the transpiled modules (packages).

	// for _, mod := range a.inoxModules {
	// 	dir := ""

	// 	if mod.name == a.mainModuleName {

	// 	}

	// 	mod.pkg.WriteTo(fls, dir)
	// }

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
