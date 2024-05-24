package inoxmod

import (
	"errors"
	"fmt"
	"path/filepath"

	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	ErrAbsoluteModuleSourcePathUsedInURLImportedModule = errors.New("absolute module source path used in module imported from URL")
)

func GetSourceFromImportSource(importSrc ResourceName, parentModule *Module, ctx Context) (ResourceName, error) {
	//if src is a relative path and the importing module has been itself imported from an URL we make an URL with the right path.

	switch {
	case importSrc.IsPath():
		importPath := importSrc.ResourceName()

		if parentModule != nil && parentModule.HasURLSource() {
			if isPathAbsolute(importPath) {
				return nil, ErrAbsoluteModuleSourcePathUsedInURLImportedModule
			}
			u := utils.MustGet(parentModule.AbsoluteSource()).ResourceName()

			dir, ok, err := getParentUrlDir(u)

			if err != nil {
				return nil, fmt.Errorf("import: impossible to resolve relative import path: %w", err)
			}
			if !ok {
				return nil, fmt.Errorf("import: impossible to resolve relative import path, parent module URL is %q", u)
			}

			return CreatePath(filepath.Join(dir, importPath)), nil
		} else {
			if isPathRelative(importPath) {
				if parentModule != nil {
					parentModulePath := utils.MustGet(parentModule.AbsoluteSource()).ResourceName()
					parentModuleDir := filepath.Dir(parentModulePath)

					importPath = filepath.Join(parentModuleDir, importPath)
				} else {
					return nil, fmt.Errorf("import: impossible to resolve relative import path as parent state has no module")
				}
			}

			importPath, err := filepath.Abs(importPath)
			if err != nil {
				return nil, fmt.Errorf("import: %w", err)
			}

			fsPerm := CreateReadFilePermission(importPath)

			if err := ctx.CheckHasPermission(fsPerm); err != nil {
				return nil, fmt.Errorf("import: %s", err.Error())
			}
			return CreatePath(importPath), nil
		}
	case importSrc.IsURL():
		url := importSrc.ResourceName()
		httpPerm := CreateHttpReadPermission(url)
		if err := ctx.CheckHasPermission(httpPerm); err != nil {
			return nil, fmt.Errorf("import: %s", err.Error())
		}
		return CreateURL(url), nil
	default:
		return nil, fmt.Errorf("import: invalid source, type is %T", importSrc)
	}
}
