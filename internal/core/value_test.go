package core

import (
	"runtime"
	"strings"
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestURLPattern(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 10)
	}

	t.Run("Test", func(t *testing.T) {
		assert.False(t, URLPattern("https://localhost:443/ab/...").Test(nil, URL("https://localhost:443/ab")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Test(nil, URL("https://localhost:443/ab/c")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Test(nil, URL("https://localhost:443/ab/c?q=a")))

		assert.False(t, URLPattern("https://localhost:443/...").Test(nil, URL("wss://localhost:443/")))
		assert.True(t, URLPattern("wss://localhost:443/...").Test(nil, URL("wss://localhost:443/")))
	})

	t.Run("Includes", func(t *testing.T) {
		//URL
		assert.False(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab/c")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab/c?q=a")))

		assert.False(t, URLPattern("https://localhost:443/...").Includes(nil, URL("wss://localhost:443/")))
		assert.True(t, URLPattern("wss://localhost:443/...").Includes(nil, URL("wss://localhost:443/")))

		//URL pattern
		assert.False(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab/c")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab/c?q=a")))

		assert.False(t, URLPattern("https://localhost:443/...").Includes(nil, URLPattern("wss://localhost:443/")))
		assert.True(t, URLPattern("wss://localhost:443/...").Includes(nil, URLPattern("wss://localhost:443/")))

		assert.False(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/...")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab/c/...")))
	})
}

func TestPathPattern(t *testing.T) {
	t.Run("", func(t *testing.T) {

		assert.True(t, PathPattern("/...").Test(nil, Path("/")))
		assert.True(t, PathPattern("/...").Test(nil, Path("/f")))
		assert.True(t, PathPattern("/...").Test(nil, Path("/file.txt")))
		assert.True(t, PathPattern("/...").Test(nil, Path("/dir/")))
		assert.True(t, PathPattern("/...").Test(nil, Path("/dir/file.txt")))

		assert.True(t, PathPattern("/dir/...").Test(nil, Path("/dir/")))
		assert.True(t, PathPattern("/dir/...").Test(nil, Path("/dir/file.txt")))
		assert.False(t, PathPattern("/dir/...").Test(nil, Path("/")))
		assert.False(t, PathPattern("/dir/...").Test(nil, Path("/f")))
		assert.False(t, PathPattern("/dir/...").Test(nil, Path("/file.txt")))

		assert.True(t, PathPattern("/*").Test(nil, Path("/")))
		assert.True(t, PathPattern("/*").Test(nil, Path("/f")))
		assert.True(t, PathPattern("/*").Test(nil, Path("/file.txt")))
		assert.False(t, PathPattern("/*").Test(nil, Path("/dir/")))
		assert.False(t, PathPattern("/*").Test(nil, Path("/dir/file.txt")))

		assert.True(t, PathPattern("/[a-z]").Test(nil, Path("/a")))
		assert.False(t, PathPattern("/[a-z]").Test(nil, Path("/aa")))
		assert.False(t, PathPattern("/[a-z]").Test(nil, Path("/a0")))
		assert.False(t, PathPattern("/[a-z]").Test(nil, Path("/0")))
		assert.False(t, PathPattern("/[a-z]").Test(nil, Path("/a/")))
		assert.False(t, PathPattern("/[a-z]").Test(nil, Path("/a/a")))

		assert.True(t, PathPattern("/**").Test(nil, Path("/")))
		assert.True(t, PathPattern("/**").Test(nil, Path("/f")))
		assert.True(t, PathPattern("/**").Test(nil, Path("/file.txt")))
		assert.True(t, PathPattern("/**").Test(nil, Path("/dir/")))
		assert.True(t, PathPattern("/**").Test(nil, Path("/dir/file.txt")))

		assert.True(t, PathPattern("/**/file.txt").Test(nil, Path("/file.txt")))
		assert.True(t, PathPattern("/**/file.txt").Test(nil, Path("/dir/file.txt")))
		assert.True(t, PathPattern("/**/file.txt").Test(nil, Path("/dir/subdir/file.txt")))
	})
}

func TestHostPatternTest(t *testing.T) {
	assert.True(t, HostPattern("https://*.com").Test(nil, Host("https://a.com")))
	assert.True(t, HostPattern("https://a*.com").Test(nil, Host("https://a.com")))
	assert.True(t, HostPattern("https://a*.com").Test(nil, Host("https://ab.com")))
	assert.False(t, HostPattern("https://*.com").Test(nil, Host("https://sub.a.com")))
	assert.True(t, HostPattern("https://**.com").Test(nil, Host("https://sub.a.com")))
	assert.True(t, HostPattern("https://a.*").Test(nil, Host("https://a.com")))
	assert.False(t, HostPattern("https://sub.*").Test(nil, Host("https://sub.a.com")))
	assert.True(t, HostPattern("https://sub.**").Test(nil, Host("https://sub.a.com")))
	assert.False(t, HostPattern("https://*.com").Test(nil, Host("://a.com")))
	assert.False(t, HostPattern("://*.com").Test(nil, Host("https://a.com")))
	assert.False(t, HostPattern("://*.com:8080").Test(nil, Host("https://a.com")))
	assert.True(t, HostPattern("://*.com").Test(nil, Host("://a.com")))

	assert.False(t, HostPattern("https://*.com").Test(nil, Host("ws://a.com")))
	assert.True(t, HostPattern("ws://*.com").Test(nil, Host("ws://a.com")))
}

func TestNamedSegmentPathPatternTest(t *testing.T) {
	res := parseEval(t, `%/home/{:username}`)
	patt := res.(*NamedSegmentPathPattern)

	for _, testCase := range []struct {
		ok   bool
		path Path
	}{
		{false, "/home"},
		{false, "/home/"},
		{true, "/home/user"},
		{false, "/home/user/"},
		{false, "/home/user/e"},
	} {
		t.Run(string(testCase.path), func(t *testing.T) {
			assert.Equal(t, testCase.ok, patt.Test(nil, testCase.path))
		})
	}
}

func TestNamedSegmentPathPatternMatchGroups(t *testing.T) {
	res1 := parseEval(t, `%/home/{:username}`)
	patt1 := res1.(*NamedSegmentPathPattern)

	for _, testCase := range []struct {
		groups map[string]Serializable
		path   Path
	}{
		{nil, "/home"},
		{nil, "/home/"},
		{map[string]Serializable{"0": Path("/home/user"), "username": Str("user")}, "/home/user"},
		{nil, "/home/user/"},
		{nil, "/home/user/e"},
	} {
		t.Run(string(testCase.path), func(t *testing.T) {
			groups, ok, err := patt1.MatchGroups(nil, testCase.path)
			assert.NoError(t, err)

			if ok != (testCase.groups != nil) {
				assert.FailNow(t, "invalid match")
			}
			assert.Equal(t, testCase.groups, groups)
		})

	}

	res2 := parseEval(t, `%/home/{:username}/`)
	patt2 := res2.(*NamedSegmentPathPattern)

	for _, testCase := range []struct {
		groups map[string]Serializable
		path   Path
	}{
		{nil, "/home"},
		{nil, "/home/"},
		{nil, "/home/user"},
		{map[string]Serializable{"0": Path("/home/user/"), "username": Str("user")}, "/home/user/"},
		{nil, "/home/user/e"},
	} {
		t.Run("pattern ends with slash, "+string(testCase.path), func(t *testing.T) {
			groups, ok, err := patt2.MatchGroups(nil, testCase.path)
			assert.NoError(t, err)

			if ok != (testCase.groups != nil) {
				assert.FailNow(t, "invalid match")
			}
			assert.Equal(t, testCase.groups, groups)
		})

	}
}

func TestRepeatedPatternElementRandom(t *testing.T) {
	t.Run("2 ocurrences of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.ExactOcurrence,
			exactCount:        2,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			assert.Equal(t, Str("aa"), patt.Random(ctx).(Str))
		}
	})

	t.Run("optional ocurrence of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.OptionalOcurrence,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			s := patt.Random(ctx).(Str)
			assert.Equal(t, Str(strings.Repeat("a", len(s))), s)
		}
	})

	t.Run("zero or more ocurrences of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.ZeroOrMoreOcurrence,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			s := patt.Random(ctx).(Str)
			length := len(s)

			assert.Equal(t, Str(strings.Repeat("a", length)), s)
		}
	})

	t.Run("at least one ocurrence of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.ZeroOrMoreOcurrence,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			s := patt.Random(ctx).(Str)
			length := len(s)

			assert.Equal(t, Str(strings.Repeat("a", length)), s)
		}
	})
}

func TestSequenceStringPatternRandom(t *testing.T) {
	ctx := NewContext(ContextConfig{})

	patt1 := SequenceStringPattern{
		regexp: nil,
		node:   nil,
		elements: []StringPattern{
			NewExactStringPattern(Str("a")),
			NewExactStringPattern(Str("b")),
		},
	}

	assert.Equal(t, Str("ab"), patt1.Random(ctx))
}

func TestUnionStringPatternRandom(t *testing.T) {
	ctx := NewContext(ContextConfig{})

	patt1 := UnionStringPattern{
		regexp: nil,
		node:   nil,
		cases: []StringPattern{
			NewExactStringPattern(Str("a")),
			NewExactStringPattern(Str("b")),
		},
	}

	for i := 0; i < 5; i++ {
		s := patt1.Random(ctx).(Str)
		assert.True(t, s == "a" || s == "b")
	}

}

// parseEval is a utility function that parses the input string and then evalutes the parsed module.
func parseEval(t *testing.T, s string) Value {

	chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
		NameString: "test",
		CodeString: s,
	}))

	mod, err := parse.ParseChunk(s, "")
	assert.NoError(t, err)
	_, err = StaticCheck(StaticCheckInput{
		State: NewGlobalState(NewContext(ContextConfig{})),
		Node:  mod,
		Chunk: chunk,
	})
	assert.NoError(t, err)

	res, err := TreeWalkEval(mod, NewTreeWalkState(NewDefaultTestContext()))
	assert.NoError(t, err)
	return res
}
