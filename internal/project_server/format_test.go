package project_server

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
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

		//properties
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

		//colon
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
	}

	for _, testCase := range cases {
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "",
			CodeString: strings.Join(testCase[0], "\n"),
		}))

		formatted := format(chunk, defines.FormattingOptions{})
		assert.Equal(t, strings.Join(testCase[1], "\n"), formatted)
	}
}
