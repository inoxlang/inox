package internal

import (
	"strings"

	core "github.com/inoxlang/inox/internal/core"

	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/mysql"
	testdriver "github.com/pingcap/tidb/parser/test_driver"
)

var (
	_ = []core.StatelessParser{&sqlQueryParser{}}
)

type sqlQueryParser struct {
	parser *parser.Parser
}

func newQueryParser() *sqlQueryParser {
	p := &sqlQueryParser{
		parser: parser.New(),
	}

	return p
}

func (p *sqlQueryParser) Validate(ctx *core.Context, s string) bool {
	_, w, err := p.parser.ParseSQL(s)
	return w == nil && err == nil
}

func (p *sqlQueryParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}

type sqlStringValueParser struct {
	parser *parser.Parser
}

func newStringValueParser() *sqlStringValueParser {
	p := &sqlStringValueParser{
		parser: parser.New(),
	}

	return p
}

func (p *sqlStringValueParser) Validate(ctx *core.Context, s string) bool {
	if strings.TrimSpace(s) != s { // TODO: change
		return false
	}

	n, err := p.parser.ParseOneStmt("select "+s+";", "", "")
	if err != nil {
		return false
	}

	switch node := n.(type) {
	case *ast.SelectStmt:
		if node.Kind != ast.SelectStmtKindSelect || node.Fields == nil || len(node.Fields.Fields) != 1 {
			return false
		}
		typ := node.Fields.Fields[0].Expr.(*testdriver.ValueExpr).Type.GetType()
		return typ == mysql.TypeVarchar || typ == mysql.TypeVarString
	default:
	}
	return false
}

func (p *sqlStringValueParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}
