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
	}

	for _, testCase := range cases {
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "",
			CodeString: strings.Join(testCase[0], "\n"),
		}))

		formatted := formatInoxChunk(chunk, defines.FormattingOptions{})
		assert.Equal(t, strings.Join(testCase[1], "\n"), formatted)
	}
}
