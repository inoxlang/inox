package internal

import (
	"fmt"
	"net/url"

	"github.com/inoxlang/inox/internal/core"
	parse "github.com/inoxlang/inox/internal/parse"
)

func _sha1(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return core.NewMutableByteSlice(_hash(arg, SHA1), "")
}

func _sha2(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return core.NewMutableByteSlice(_hash(arg, MD5), "")
}

func _mkpath(ctx *core.Context, arg core.Value) core.Path {
	switch a := arg.(type) {
	case *core.List:
		pth := ""
		for i := 0; i < a.Len(); i++ {
			pth += fmt.Sprint(a.At(ctx, i))
		}
		//TODO: add additionnal checks
		if !parse.HasPathLikeStart(pth) {
			panic(fmt.Errorf("mkpath: invalid path '%s'", pth))
		}

		path, ok := parse.ParsePath(pth)
		if !ok {
			panic(fmt.Errorf("mkpath: invalid path '%s'", pth))
		}

		return core.Path(path)
	}
	panic(fmt.Errorf("mkpath: a list is expected, not a(n) %T", arg))
}

func _make_path_pattern(ctx *core.Context, arg core.Value) core.PathPattern {
	switch a := arg.(type) {
	case *core.List:
		patt := ""
		for i := 0; i < a.Len(); i++ {
			patt += fmt.Sprint(a.At(ctx, i))
		}
		if !parse.HasPathLikeStart(patt) {
			panic(fmt.Errorf("make_path_pattern: invalid path pattern '%s'", patt))
		}

		if !parse.ParsePathPattern(patt) {

			panic(fmt.Errorf("make_path_pattern: invalid path pattern '%s'", patt))
		}

		return core.PathPattern(patt)
	}
	panic(fmt.Errorf("mkpath: a list is expected, not a(n) %T", arg))
}

func _mkurl(ctx *core.Context, arg core.Value) core.URL {
	switch a := arg.(type) {
	case *core.List:
		u := ""
		for i := 0; i < a.Len(); i++ {
			u += fmt.Sprint(a.At(ctx, i))
		}

		if _, err := url.Parse(u); err != nil {
			panic(fmt.Errorf("mkurl: invalid core.URL '%s': %s", u, err))
		}

		if _, ok := parse.ParseURL(u); !ok {
			panic(fmt.Errorf("mkurl: invalid core.URL '%s'", u))
		}

		return core.URL(u)
	}
	panic(fmt.Errorf("mkurl: a list is expected, not a(n) %T", arg))
}

func newInsecure() *core.Namespace {
	return core.NewNamespace("insecure", map[string]core.Value{
		"sha1":              core.ValOf(_sha1),
		"md5":               core.ValOf(_sha2),
		"mkpath":            core.ValOf(_mkpath),
		"make_path_pattern": core.ValOf(_make_path_pattern),
		"mkurl":             core.ValOf(_mkurl),
	})
}
