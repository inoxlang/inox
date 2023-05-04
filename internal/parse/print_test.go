package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrint(t *testing.T) {

	testCases := []string{
		"manifest {}",
		"manifest {",
		"manifest",
		"manifest ",
		//simple literals
		"1",
		" 1",
		"1x",
		"1x/s",
		"https://example.com",
		"https://example.com/",
		"1..2",
		"1..",
		"'a'..'b'",
		"'a'..",
		//url expressions
		"https://{host}/",
		"https://example.com/{x}",
		"https://example.com/{",
		"https://example.com/{\n",
		"https://example.com/{x",
		"https://example.com/?x={1}",
		"https://example.com/?x={",
		"https://example.com/?x={\n",
		"https://example.com/?x={1",
		"https://example.com/?x={1}&",
		"https://example.com/?x={1}&&",
		"https://example.com/?x={1}&y=2",
		"https://example.com/?x={1}&&y=2",
		"https://example.com/?x={1}&=&y=2",
		"@site/",
		"#a",
		//variable
		"(f)",
		"a",
		"a?",
		//assignment
		"a = 0",
		"assign a b = c",
		// member expression
		"a.b",
		"a.b.",
		"a.b?",
		"$a.b",
		"$a.b.",
		//object
		"{}",
		"{ }",
		`{"a":1}`,
		`{"a" :1}`,
		`{"a": 1}`,
		//record
		"#{}",
		"#{ }",
		`#{"a":1}`,
		`#{"a" :1}`,
		`#{"a": 1}`,
		//call
		"f()",
		"f(1)",
		"f(1,2)",
		"f",
		"f 1",
		"f 1 2",
		"a = f(1 2)",
		//pipe
		"f 1 | g 2",
		"f 1 | g 2 | h 3",
		"a = | f 1 | g 2",
		//binary expression
		"(a + b)",
		"(a - b)",
		"(a * b)",
		"(a / b)",
		"(a < b)",
		"(a <= b)",
		"(a > b)",
		"(a >= b)",
		"(a + b)",
		"(a - b)",
		"(a * b)",
		"(a / b)",
		"(a < b)",
		"(a <= b)",
		"(a > b)",
		"(a >= b)",
		"(a == b)",
		"(a is b)",
		"(a is-not b)",
		"(a in b)",
		"(a not-in b)",
		"(a keyof b)",
		"(a match b)",
		"(a not-match b)",
		//lists
		"[]",
		"[,]",
		"[,",
		".{",
		".{,",
		".{,}",
		//patterns
		"%",
		"%a",
		"%a.",
		"%a.b",
		"%a?",
		"%{}",
		"%{a:1}",
		"%str('a')",
		"%str('a' 'b')",
		`%str((| "a"))`,
		`%str((| "a" | "b" ))`,
		"%``",
		"%`a`",
		"%`é`",
		"%`\n`",
		"%`\\``",
		"%`",
		"%`a",
		"%/",
		"%/...",
		"%/*",
		"%/{:name}",
		"%/{",
		"%/{\n",
		"%/{:",
		"%/{:\n",
		"%/{:name",
		"%/{:name\n",
		"%/{name}",
		"%/{name",
		"%/{name\n",
		"%|",
		"%| 1",
		"%| 1 |",
		"%| 1 | 2",
		//string template literals
		"%p``",
		"%p`",
		"%p`{{int:a}}`",
		"%p`{{int:a}}",
		"%p`{{int:a",
		"%p`{{int:",
		"%p`{{int",
		"%p`{{",
		"%https://**",
		"%https://example.com/...",
		"%https://**.example.com",
		//udata literal
		"udata",
		"udata 0",
		"udata 0 {}",
		"udata 0 {",
		"udata {}",
		"udata {",
		"udata { 0 {} }",
		"udata { 0 { }",
		"udata { 0 { ",
		"udata { 0 ",
		"udata { 0 {}, }",
		"udata { 0 {}, 1}",
		"udata { 0 {1, 2}, 3}",
		//spawn expression
		"go {} do",
		"go {} do {}",
		//mapping expression
		"Mapping {}",
		"Mapping { }",
		"Mapping",
		"Mapping {",
		//switch statement
		"switch",
		"switch 1",
		"switch 1 {",
		"switch 1 {}",
		"switch 1 { 1 }",
		"switch 1 { 1 {}",
		"switch 1 { 1 {",
		"switch 1 { 1 {} 2 {}",
		"switch 1 { 1 {} 2 {} }",
		"switch 1 { 1, 2 {} }",
		//match statement
		"match",
		"match 1",
		"match 1 {",
		"match 1 {}",
		"match 1 { 1 }",
		"match 1 { 1 {}",
		"match 1 { 1 {",
		"match 1 { 1 {} 2 {}",
		"match 1 { 1 {} 2 {} }",
		"match 1 { 1, 2 {} }",
		//function expressions
		"fn(){}",
		"fn(arg){}",
		"fn(arg %int){}",
		//others
		"@(1)",
	}

	n, _ := ParseChunk("%/", "")
	_ = SPrint(n, PrintConfig{})

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			n, _ := ParseChunk(testCase, "")
			s := SPrint(n, PrintConfig{})
			assert.Equal(t, testCase, s)
		})
	}

}
