package css

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {

	stylesheet, err := ParseString(context.Background(), `
		/* comment */

		@import "style.css";

		@media screen {
			div {
				width: 5px;
			}
		}

		@media screen and (min-width: 400px) {}

		.div {
			width: 6px;
		}
		.div [a] {
			background: rgb(5, 5, 5);
		}

		:root {
			--primary-bg: white;
		}
	`)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, Stylesheet, stylesheet.Type) {
		return
	}

	assert.Empty(t, stylesheet.Data)
	if !assert.Len(t, stylesheet.Children, 7) {
		return
	}

	//Check comment
	comment := stylesheet.Children[0]
	if !assert.Equal(t, Comment, comment.Type) {
		return
	}
	assert.Equal(t, "/* comment */", comment.Data)
	assert.Empty(t, comment.Children)

	//Check at-rule
	atRule := stylesheet.Children[1]
	if !assert.Equal(t, AtRule, atRule.Type) {
		return
	}

	assert.Equal(t, "@import", atRule.Data)
	if !assert.Len(t, atRule.Children, 1) {
		return
	}

	assert.Equal(t, Node{Type: String, Data: "\"style.css\""}, atRule.Children[0])

	//Check first at-rule wih media query

	atRule = stylesheet.Children[2]
	if !assert.Equal(t, AtRule, atRule.Type) {
		return
	}
	assert.Equal(t, "@media", atRule.Data)

	if !assert.Len(t, atRule.Children, 2) {
		return
	}

	{
		//Check media query
		query := atRule.Children[0]
		if !assert.Equal(t, MediaQuery, query.Type) {
			return
		}
		assert.Empty(t, query.Data)
		if !assert.Len(t, query.Children, 1) {
			return
		}

		assert.Equal(t, Node{Type: Ident, Data: "screen"}, query.Children[0])

		//Check ruleset
		ruleset := atRule.Children[1]
		if !assert.Equal(t, Ruleset, ruleset.Type) {
			return
		}
		assert.Empty(t, ruleset.Data)
		if !assert.Len(t, ruleset.Children, 2) {
			return
		}

		assert.Equal(t, Node{
			Type: Selector,
			Children: []Node{
				{
					Type: Ident,
					Data: "div",
				},
			},
		}, ruleset.Children[0])

		assert.Equal(t, Node{
			Type: Declaration,
			Data: "width",
			Children: []Node{
				{Type: Dimension, Data: "5px"},
			},
		}, ruleset.Children[1])
	}

	//Check second at-rule wih media query

	atRule = stylesheet.Children[3]
	if !assert.Equal(t, AtRule, atRule.Type) {
		return
	}
	assert.Equal(t, "@media", atRule.Data)

	if !assert.Len(t, atRule.Children, 1) {
		return
	}

	{
		//Check media query
		query := atRule.Children[0]
		if !assert.Equal(t, MediaQuery, query.Type) {
			return
		}
		assert.Empty(t, query.Data)
		if !assert.Len(t, query.Children, 3) {
			return
		}

		screenIdent := query.Children[0]
		assert.Equal(t, Node{Type: Ident, Data: "screen"}, screenIdent)

		andIdent := query.Children[1]
		assert.Equal(t, Node{Type: Ident, Data: "and"}, andIdent)

		mediaFeature := query.Children[2]

		if !assert.Equal(t, Node{
			Type: MediaFeature,
			Data: "min-width",
			Children: []Node{
				{
					Type: Dimension,
					Data: "400px",
				},
			},
		}, mediaFeature) {
			return
		}
	}

	//Check second ruleset
	ruleset := stylesheet.Children[4]
	if !assert.Equal(t, Ruleset, ruleset.Type) {
		return
	}
	assert.Empty(t, ruleset.Data)
	if !assert.Len(t, ruleset.Children, 2) {
		return
	}

	assert.Equal(t, Node{
		Type: Selector,
		Children: []Node{
			{
				Type: ClassName,
				Data: ".div",
			},
		},
	}, ruleset.Children[0])

	assert.Equal(t, Node{
		Type: Declaration,
		Data: "width",
		Children: []Node{
			{Type: Dimension, Data: "6px"},
		},
	}, ruleset.Children[1])

	//Check third ruleset
	ruleset = stylesheet.Children[5]
	if !assert.Equal(t, Ruleset, ruleset.Type) {
		return
	}
	assert.Empty(t, ruleset.Data)
	if !assert.Len(t, ruleset.Children, 2) {
		return
	}

	assert.Equal(t, Node{
		Type: Selector,
		Children: []Node{
			{
				Type: ClassName,
				Data: ".div",
			},
			{
				Type: Whitespace,
				Data: " ",
			},
			{
				Type: AttributeSelector,
				Children: []Node{
					{
						Type: Ident,
						Data: "a",
					},
				},
			},
		},
	}, ruleset.Children[0])

	decl := ruleset.Children[1]

	assert.Equal(t, Node{
		Type: Declaration,
		Data: "background",
		Children: []Node{
			{
				Type: FunctionCall,
				Data: "rgb",
				Children: []Node{
					{
						Type: Number,
						Data: "5",
					},
					{
						Type: Number,
						Data: "5",
					},
					{
						Type: Number,
						Data: "5",
					},
				},
			},
		},
	}, decl)

	//Check last ruleset

	ruleset = stylesheet.Children[6]
	if !assert.Equal(t, Ruleset, ruleset.Type) {
		return
	}
	assert.Empty(t, ruleset.Data)
	if !assert.Len(t, ruleset.Children, 2) {
		return
	}

	assert.Equal(t, Node{
		Type: Selector,
		Children: []Node{
			{
				Type: PseudoClassSelector,
				Data: ":root",
			},
		},
	}, ruleset.Children[0])

	decl = ruleset.Children[1]

	assert.Equal(t, Node{
		Type: CustomProperty,
		Data: "--primary-bg",
		Children: []Node{
			{
				Type: CustomPropertyValue,
				Data: " white",
			},
		},
	}, decl)
}
