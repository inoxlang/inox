package projectserver

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {

	cases := [][2][]string{
		{
			{` manifest {}`},
			{`manifest {}`},
		},
		{
			{` const ()`},
			{`const ()`},
		},
		{
			{
				"const (",
				"a = 1",
				")",
			},
			{
				"const (",
				"\ta = 1",
				")",
			},
		},
		{
			{
				"const (",
				"a = 1",
				"b = 2",
				")",
			},
			{
				"const (",
				"\ta = 1",
				"\tb = 2",
				")",
			},
		},
		{
			{
				"manifest {}",
				"\ta = 1",
			},
			{
				"manifest {}",
				"a = 1",
			},
		},
		{
			{

				"manifest {}",
				"fn f(){",
				"a = 1",
				"}",
			},
			{
				"manifest {}",
				"fn f(){",
				"\ta = 1",
				"}",
			},
		},

		//several statements on the same line (top level)
		{
			{
				"a = 1; b = 2",
			},
			{
				"a = 1; b = 2",
			},
		},
		{
			{
				"manifest {}",
				"a = 1;b = 2",
			},
			{
				"manifest {}",
				"a = 1; b = 2",
			},
		},
		//several statements on the same line in a function
		{
			{
				"manifest {}",
				"fn f(){",
				"\ta = 1; b = 2",
				"}",
			},
			{
				"manifest {}",
				"fn f(){",
				"\ta = 1; b = 2",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"fn f(){",
				"\ta = 1;b = 2",
				"}",
			},
			{
				"manifest {}",
				"fn f(){",
				"\ta = 1; b = 2",
				"}",
			},
		},
		//properties of a top level object-like literal
		{
			{
				"manifest {}",
				"a = {",
				"prop: 1",
				"}",
			},
			{
				"manifest {}",
				"a = {",
				"\tprop: 1",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"a = #{",
				"prop: 1",
				"}",
			},
			{
				"manifest {}",
				"a = #{",
				"\tprop: 1",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"a = %{",
				"prop: 1",
				"}",
			},
			{
				"manifest {}",
				"a = %{",
				"\tprop: 1",
				"}",
			},
		},

		{
			{
				"manifest {}",
				"a = {",
				"1",
				"}",
			},
			{
				"manifest {}",
				"a = {",
				"\t1",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"a = {",
				"inner: {",
				"prop: {}",
				"}",
				"}",
			},
			{
				"manifest {}",
				"a = {",
				"\tinner: {",
				"\t\tprop: {}",
				"\t}",
				"}",
			},
		},

		//properties of a an object-like literal in a function
		{
			{
				"manifest {}",
				"fn(){",
				"\ta = {",
				"\tprop: 1",
				"\t}",
				"}",
			},
			{
				"manifest {}",
				"fn(){",
				"\ta = {",
				"\t\tprop: 1",
				"\t}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"fn(){",
				"\ta = #{",
				"\tprop: 1",
				"\t}",
				"}",
			},
			{
				"manifest {}",
				"fn(){",
				"\ta = #{",
				"\t\tprop: 1",
				"\t}",
				"}",
			},
		},

		{
			{
				"manifest {}",
				"fn(){",
				"\ta = %{",
				"\tprop: 1",
				"\t}",
				"}",
			},
			{
				"manifest {}",
				"fn(){",
				"\ta = %{",
				"\t\tprop: 1",
				"\t}",
				"}",
			},
		},

		{
			{
				"manifest {}",
				"fn(){",
				"\ta = {",
				"\t1",
				"\t}",
				"}",
			},
			{
				"manifest {}",
				"fn(){",
				"\ta = {",
				"\t\t1",
				"\t}",
				"}",
			},
		},

		{
			{
				"manifest {}",
				"fn(){",
				"\ta = {",
				"\tinner: {",
				"\tprop: {}",
				"\t}",
				"\t}",
				"}",
			},
			{
				"manifest {}",
				"fn(){",
				"\ta = {",
				"\t\tinner: {",
				"\t\t\tprop: {}",
				"\t\t}",
				"\t}",
				"}",
			},
		},

		//colon in a top level object-like literal
		{
			{
				"manifest {}",
				"a = {",
				"\tprop : 1",
				"}",
			},
			{
				"manifest {}",
				"a = {",
				"\tprop: 1",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"a = #{",
				"\tprop : 1",
				"}",
			},
			{
				"manifest {}",
				"a = #{",
				"\tprop: 1",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"a = %{",
				"\tprop : 1",
				"}",
			},
			{
				"manifest {}",
				"a = %{",
				"\tprop: 1",
				"}",
			},
		},

		//list literal
		{
			{
				"manifest {}",
				"[",
				"1",
				"]",
			},
			{
				"manifest {}",
				"[",
				"\t1",
				"]",
			},
		},

		//switch
		{
			{
				"manifest {}",
				"switch 1 {",
				"0 {}",
				"}",
			},
			{
				"manifest {}",
				"switch 1 {",
				"\t0 {}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"switch 1 {",
				"\t0 {}",
				"}",
			},
			{
				"manifest {}",
				"switch 1 {",
				"\t0 {}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"switch 1 {",
				"defaultcase {}",
				"}",
			},
			{
				"manifest {}",
				"switch 1 {",
				"\tdefaultcase {}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"switch 1 {",
				"\tdefaultcase {}",
				"}",
			},
			{
				"manifest {}",
				"switch 1 {",
				"\tdefaultcase {}",
				"}",
			},
		},

		//match

		{
			{
				"manifest {}",
				"match 1 {",
				"0 {}",
				"}",
			},
			{
				"manifest {}",
				"match 1 {",
				"\t0 {}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"match 1 {",
				"\t0 {}",
				"}",
			},
			{
				"manifest {}",
				"match 1 {",
				"\t0 {}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"match 1 {",
				"defaultcase {}",
				"}",
			},
			{
				"manifest {}",
				"match 1 {",
				"\tdefaultcase {}",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"match 1 {",
				"\tdefaultcase {}",
				"}",
			},
			{
				"manifest {}",
				"match 1 {",
				"\tdefaultcase {}",
				"}",
			},
		},

		//pattern definition
		{
			{
				"manifest {}",
				"\tpattern p = {",
				"}",
			},
			{
				"manifest {}",
				"pattern p = {",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"pattern p = {",
				"}",
			},
			{
				"manifest {}",
				"pattern p = {",
				"}",
			},
		},

		//pattern namespace definition
		{
			{
				"manifest {}",
				"\tpnamespace ns. = {",
				"}",
			},
			{
				"manifest {}",
				"pnamespace ns. = {",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"pnamespace ns. = {",
				"}",
			},
			{
				"manifest {}",
				"pnamespace ns. = {",
				"}",
			},
		},

		//function definition
		{
			{
				"manifest {}",
				"\tfn f(){",
				"}",
			},
			{
				"manifest {}",
				"fn f(){",
				"}",
			},
		},
		{
			{
				"manifest {}",
				"fn f(){",
				"}",
			},
			{
				"manifest {}",
				"fn f(){",
				"}",
			},
		},

		//assignment

		{
			{
				"manifest {}",
				"a=1",
			},
			{
				"manifest {}",
				"a = 1",
			},
		},

		{
			{
				"manifest {}",
				"a= 1",
			},
			{
				"manifest {}",
				"a = 1",
			},
		},
		{
			{
				"manifest {}",
				"a =1",
			},
			{
				"manifest {}",
				"a = 1",
			},
		},

		//xml attributes

		{
			{
				"manifest {}",
				"html<div a=1/>",
			},
			{
				"manifest {}",
				"html<div a=1/>",
			},
		},
		{
			{
				"manifest {}",
				"html<div a=1></div>",
			},
			{
				"manifest {}",
				"html<div a=1></div>",
			},
		},

		//single-line xml interpolations
		{
			{
				"manifest {}",
				"html<div>{1}</div>",
			},
			{
				"manifest {}",
				"html<div>{1}</div>",
			},
		},

		//multi-line xml interpolations should not be updated
		{
			{
				"manifest {}",
				"html<div>",
				"{",
				"1",
				"}",
				"</div>",
			},
			{
				"manifest {}",
				"html<div>",
				"{",
				"1",
				"}",
				"</div>",
			},
		},
		{
			{
				"manifest {}",
				"html<div>",
				"\t{",
				"\t1",
				"\t}",
				"</div>",
			},
			{
				"manifest {}",
				"html<div>",
				"\t{",
				"\t1",
				"\t}",
				"</div>",
			},
		},
		{
			{
				"manifest {}",
				"html<div>",
				"{",
				"\t1",
				"}",
				"</div>",
			},
			{
				"manifest {}",
				"html<div>",
				"{",
				"\t1",
				"}",
				"</div>",
			},
		},
		//comments
		{
			{
				"manifest {}",
				"# comment",
			},
			{
				"manifest {}",
				"# comment",
			},
		},
		{
			{
				"manifest {}",
				" # comment",
			},
			{
				"manifest {}",
				"# comment",
			},
		},
		{
			{
				"manifest {}",
				"a = {",
				"\t# comment",
				"\ta: 1",
				"\t# comment",
				"\tb: 1",
				"}",
			},
			{
				"manifest {}",
				"a = {",
				"\t# comment",
				"\ta: 1",
				"\t# comment",
				"\tb: 1",
				"}",
			},
		},
		//formatting of comments inside markup interpolations is not supported yet:
		{
			{
				"manifest {}",
				"html<div>",
				"\t{",
				"\t\tfn(){",
				"\t\t# comment",
				"\t\t}",
				"\t}",
				"</div>",
			},
			{
				"manifest {}",
				"html<div>",
				"\t{",
				"\t\tfn(){",
				"\t\t# comment",
				"\t\t}",
				"\t}",
				"</div>",
			},
		},
	}

	formatter := formatter{}

	for _, testCase := range cases {
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "",
			CodeString: strings.Join(testCase[0], "\n"),
		}))

		formatted := formatter.formatInoxChunk(chunk, defines.FormattingOptions{})
		if !assert.Equal(t, strings.Join(testCase[1], "\n"), formatted, "input: "+formatted) {
			formatter.formatInoxChunk(chunk, defines.FormattingOptions{})
		}
	}
}
