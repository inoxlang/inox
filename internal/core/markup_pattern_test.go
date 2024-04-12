package core_test

import (
	"runtime/debug"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestMarkupPattern(t *testing.T) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	type patternTestCases struct {
		markup      string
		shouldMatch bool
	}

	cases := map[string][]patternTestCases{
		"<div></div>": {
			{
				markup:      "<div></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div> </div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1</div>",
				shouldMatch: false,
			},
		},
		"<div>*</div>": {
			{
				markup:      "<div></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div> </div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n1\n</div>",
				shouldMatch: true,
			},
		},
		"<div>1*</div>": {
			{
				markup:      "<div>1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>12</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n1\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>12<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a>2</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: false,
			},
		},
		"<div>*1</div>": {
			{
				markup:      "<div>1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n1\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div>12</div>",
				shouldMatch: false,
			},
		},
		"<div><a></a></div>": {
			{
				markup:      "<div><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a x=1></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a><a></a></div>",
				shouldMatch: false,
			},
		},
		"<div><a+></a></div>": {
			{
				markup:      "<div><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a x=1></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: false,
			},
		},
		"<div><a*></a></div>": {
			{
				markup:      "<div></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a x=1></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a>\n</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1</div>",
				shouldMatch: false,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: false,
			},
		},
		"<div><a?></a></div>": {
			{
				markup:      "<div></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a x=1></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div>1</div>",
				shouldMatch: false,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: false,
			},
		},
		"<div>*<a></a></div>": {
			{
				markup:      "<div><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><span></span><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><span></span>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a></a>\n<a></a></div>",
				shouldMatch: false,
			},
		},
		"<div><a></a>*</div>": {
			{
				markup:      "<div><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>1</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><span></span></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n<span></span>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a><a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a></a>\n<a></a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>1<a></a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><span></span><a></a></div>",
				shouldMatch: false,
			},
		},
		"<div><a>1</a></div>": {
			{
				markup:      "<div><a>1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a>1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a>1</a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a>\n1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a>1\n</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>0<a>1</a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a>1</a>2</div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a>1</a><a></a>/div>",
				shouldMatch: false,
			},
		},
		"<div>*<a>1</a></div>": {
			{
				markup:      "<div><a>1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a>1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>\n<a>1</a>\n</div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a>\n1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a>1\n</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div>0<a>1</a></div>",
				shouldMatch: true,
			},
			{
				markup:      "<div><a>1</a><a>1</a></div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a>1</a>2</div>",
				shouldMatch: false,
			},
			{
				markup:      "<div><a>1</a><a></a>/div>",
				shouldMatch: false,
			},
		},
	}

	for pattern, patternTestCases := range cases {
		for _, testCase := range patternTestCases {

			caseName := ""
			if testCase.shouldMatch {
				caseName = testCase.markup + " should match " + pattern
			} else {
				caseName = testCase.markup + " should not match " + pattern
			}

			t.Run(caseName, func(t *testing.T) {
				var node = utils.Must(html_ns.ParseSingleNodeHTML(testCase.markup))
				var _ core.MarkupNode = node

				patternExpr := parse.MustParseExpression("%" + pattern).(*parse.MarkupPatternExpression)
				pattern, err := core.NewMarkupPatternFromExpression(patternExpr, core.StateBridge{})
				if !assert.NoError(t, err) {
					return
				}

				doMatch := func() (result bool) {
					defer func() {
						if e := recover(); e != nil {
							t.Log(t, "", string(debug.Stack()))
						}
					}()

					return pattern.Test(ctx, node)
				}

				if !assert.Equal(t, testCase.shouldMatch, doMatch()) {
					_ = pattern.Test(ctx, node)
				}
			})
		}
	}
}
