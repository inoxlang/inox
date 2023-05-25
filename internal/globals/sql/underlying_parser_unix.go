//go:build unix

package internal

import (
	"strings"

	core "github.com/inoxlang/inox/internal/core"

	//mysql
	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/mysql"
	testdriver "github.com/pingcap/tidb/parser/test_driver"

	//postgres

	pgparser "github.com/auxten/postgresql-parser/pkg/sql/parser"
)

var (
	_ = []core.StatelessParser{&sqlQueryParser{}}
)

type underlyingQueryParser struct {
	mysql *parser.Parser
}

func newUnderlyingQueryParser() *underlyingQueryParser {
	p := &underlyingQueryParser{
		mysql: parser.New(),
	}

	return p
}

func (p *underlyingQueryParser) Validate(ctx *core.Context, s string) bool {
	stmts, err := pgparser.Parse(s)
	if err != nil || len(stmts) != 1 {
		return false
	}

	_, w, err := p.mysql.ParseSQL(s)
	if w != nil || err != nil {
		return false
	}

	return true
}

func (p *underlyingQueryParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}

type underlyingStringValueParser struct {
	mysqlparser *parser.Parser
}

func newUnderlyingStringValueParser() *underlyingStringValueParser {
	p := &underlyingStringValueParser{
		mysqlparser: parser.New(),
	}

	return p
}

func (p *underlyingStringValueParser) Validate(ctx *core.Context, s string) bool {
	if strings.TrimSpace(s) != s { // TODO: change
		return false
	}

	n, err := p.mysqlparser.ParseOneStmt("select "+s+";", "", "")
	if err != nil {
		return false
	}

	switch node := n.(type) {
	case *ast.SelectStmt:
		if node.Kind != ast.SelectStmtKindSelect || node.Fields == nil || len(node.Fields.Fields) != 1 {
			return false
		}

		valueExpr, ok := node.Fields.Fields[0].Expr.(*testdriver.ValueExpr)
		if !ok {
			return false
		}
		typ := valueExpr.Type.GetType()
		return typ == mysql.TypeVarchar || typ == mysql.TypeVarString
	default:
	}
	return false
}

func (p *underlyingStringValueParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}
