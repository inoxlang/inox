package core

import (
	"fmt"
	"io"
	"strings"

	goast "go/ast"

	"github.com/inoxlang/inox/internal/core/golang/gen"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
)

type ModuleTranspilationParams struct {
	Module          *Module
	SymbolicData    *SymbolicData
	StaticCheckData *StaticCheckData
	Config          ModuleTranspilationConfig
}

type ModuleTranspilationConfig struct {
}

type Transpiler struct {
	//input
	module          *Module
	symbolicData    *SymbolicData
	staticCheckData *StaticCheckData
	config          ModuleTranspilationConfig

	//state
	chunkStack []*parse.ParsedChunkSource //main chunk + included chunks
	pkg        *gen.Pkg
	file       *gen.File     //current file
	fnDecl     *gen.FuncDecl //current function

	//output
	result *TranspiledModule

	//trace
	trace  io.Writer
	indent int
}

func TranspileModule(args ModuleTranspilationParams) (*TranspiledModule, error) {
	transpiler := &Transpiler{
		//input
		module:          args.Module,
		symbolicData:    args.SymbolicData,
		staticCheckData: args.StaticCheckData,
		config:          args.Config,

		//state
		pkg: gen.NewPkg("main"),
	}

	return transpiler.transpileModule()
}

func (t *Transpiler) transpileModule() (*TranspiledModule, error) {

	err := t.transpileMainChunk()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (t *Transpiler) transpileMainChunk() error {

	if t.trace != nil {
		t.printTrace("ENTER MAIN CHUNK")
		defer t.printTrace("LEAVE MAIN CHUNK")
	}

	mainChunk := t.module.MainChunk

	t.chunkStack = append(t.chunkStack, mainChunk)
	defer func() {
		t.chunkStack = t.chunkStack[:len(t.chunkStack)-1]
	}()

	t.file = gen.NewFileHelper(t.pkg.Name())
	t.fnDecl = gen.NewFuncDeclHelper(inoxconsts.MODULE_EXECUTION_GO_FN)

	rootNode := mainChunk.Node

	//compile constants
	if rootNode.GlobalConstantDeclarations != nil {
		decl, err := t.transpileNode(rootNode.GlobalConstantDeclarations)
		if err != nil {
			return err
		}
		t.file.AddDecl(decl.(goast.Decl))
	}

	//Transpile statements
	switch len(rootNode.Statements) {
	case 0:
		t.fnDecl.AddStmt(gen.Ret(gen.Nil))
	case 1:
		stmt, err := t.transpileNode(rootNode.Statements[0])
		if err != nil {
			return err
		}
		t.fnDecl.AddStmt(stmt.(goast.Stmt))
	default:
		for _, stmt := range rootNode.Statements {
			stmt, err := t.transpileNode(stmt)
			if err != nil {
				return err
			}
			t.fnDecl.AddStmt(stmt.(goast.Stmt))
		}
	}

	t.file.AddDecl(t.fnDecl.Node())

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
