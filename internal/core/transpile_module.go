package core

import (
	"fmt"
	goast "go/ast"
	"path/filepath"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/inoxlang/inox/internal/core/golang/gen"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

type moduleTranspilationState struct {
	//input
	moduleName      ResourceName
	module          *Module
	staticCheckData *StaticCheckData
	symbolicData    *symbolic.Data

	//actual state
	chunkStack      []*parse.ParsedChunkSource //main chunk + included chunks
	pkg             *gen.Pkg
	pkgID           string        //example: github.com/inoxlang/inox/app/routes/index_ix
	relativePkgPath string        //example: app/routes/index_ix
	file            *gen.File     //current file
	fnDecl          *gen.FuncDecl //current function

	//output
	transpiledModule *TranspiledModule

	endChan chan error //nil error on sucess
}

func (t *Transpiler) newModuleTranspilationState(resourceName ResourceName, prepared *PreparationCacheEntry) (*moduleTranspilationState, error) {
	module, staticCheckData, symbolicData, finalSymbolicCheckErr := prepared.content()
	if finalSymbolicCheckErr != nil {
		return nil, fmt.Errorf("module %s has an error: %w", resourceName.UnderlyingString(), finalSymbolicCheckErr)
	}
	if len(staticCheckData.Errors()) != 0 {
		return nil, fmt.Errorf("module %s has errors: %w", resourceName.UnderlyingString(), staticCheckData.CombinedErrors())
	}

	//Determine the package's ID and name

	pkgId := inoxconsts.MAIN_INOX_MOD_PKG_ID
	relativePgkPath := inoxconsts.RELATIVE_MAIN_INOX_MOD_PKG_PATH
	pkgName := "main"

	if t.mainModule != resourceName {
		//resource name example: /routes/users/:user-id/POST.ix

		var subPackageNameRunes []rune
		for _, r := range strings.ToLower(resourceName.UnderlyingString()) {
			switch r {
			case '_', '-', '.', ':':
				subPackageNameRunes = append(subPackageNameRunes, '_')
			default:
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					return nil, fmt.Errorf(
						"failed to generate Go package ID from module name %s: character '%s' is not supported",
						resourceName.UnderlyingString(),
						string(r),
					)
				}
				subPackageNameRunes = append(subPackageNameRunes, r)
			}
		}
		relativePgkPath = inoxconsts.RELATIVE_MAIN_INOX_MOD_PKG_PATH + "/" + string(subPackageNameRunes)
		pkgId = inoxconsts.MAIN_INOX_MOD_PKG_ID + "/" + string(subPackageNameRunes)
		pkgName = filepath.Base(pkgId)
	}

	//Return a new transpilation state.

	state := &moduleTranspilationState{
		moduleName:      resourceName,
		endChan:         make(chan error, 1),
		module:          module,
		staticCheckData: staticCheckData,
		symbolicData:    symbolicData,
		pkgID:           pkgId,
		pkg:             gen.NewPkg(pkgName),
		relativePkgPath: relativePgkPath,
	}

	return state, nil
}

// transpileModule transpiles a Module into a Golang package.
func (t *Transpiler) transpileModule(state *moduleTranspilationState) {

	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())
			t.logger.Err(err).Send()
			state.endChan <- err
		} else {
			state.endChan <- nil
		}
	}()

	err := t.transpileMainChunk(state)
	if err != nil {
		state.endChan <- err
		return
	}

	result := &TranspiledModule{
		name:         state.moduleName,
		sourceModule: state.module,
		pkg:          state.pkg,
		pkgID:        state.pkgID,
	}

	state.transpiledModule = result

	state.endChan <- nil
}

func (t *Transpiler) transpileMainChunk(state *moduleTranspilationState) error {

	if t.trace != nil {
		t.printTrace("ENTER MAIN CHUNK")
		defer t.printTrace("LEAVE MAIN CHUNK")
	}

	mainChunk := state.module.MainChunk

	state.chunkStack = append(state.chunkStack, mainChunk)
	defer func() {
		state.chunkStack = state.chunkStack[:len(state.chunkStack)-1]
	}()

	state.file = gen.NewFile(state.pkg.Name())

	rootNode := mainChunk.Node

	//compile constants
	if rootNode.GlobalConstantDeclarations != nil {
		decl, err := t.transpileNode(rootNode.GlobalConstantDeclarations)
		if err != nil {
			return err
		}
		state.file.AddDecl(decl.(goast.Decl))
	}

	state.fnDecl = gen.NewFuncDeclHelper(inoxconsts.TRANSPILED_MOD_EXECUTION_FN)

	//Transpile statements
	switch len(rootNode.Statements) {
	case 0:
		state.fnDecl.AddStmt(gen.Ret(gen.Nil))
	case 1:
		stmt, err := t.transpileNode(rootNode.Statements[0])
		if err != nil {
			return err
		}
		state.fnDecl.AddStmt(stmt.(goast.Stmt))
	default:
		for _, stmt := range rootNode.Statements {
			stmt, err := t.transpileNode(stmt)
			if err != nil {
				return err
			}
			state.fnDecl.AddStmt(stmt.(goast.Stmt))
		}
	}

	state.file.AddDecl(state.fnDecl.Node())
	state.pkg.AddFile(inoxconsts.PRIMARY_TRANSPILED_MOD_FILENAME, state.file.F)

	return nil
}

func (t *Transpiler) transpileIncludedChunk(chunk *IncludedChunk) (*TranspiledModule, error) {

	return nil, nil
}

func (t *Transpiler) transpileNode(n parse.Node) (goast.Node, error) {

	switch n.(type) {
	case *parse.IntLiteral:
		return nil, nil
	}

	return nil, fmt.Errorf("cannot compile %T", n)
}

func (t *Transpiler) printTrace(a ...any) {
	var (
		dots = strings.Repeat(". ", 31)
		n    = len(dots)
	)

	i := 2 * t.indent
	for i > n {
		fmt.Fprint(t.trace, dots)
		i -= n
	}

	fmt.Fprint(t.trace, dots[0:i])
	fmt.Fprintln(t.trace, a...)
}
