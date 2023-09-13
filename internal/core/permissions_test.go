package core

import (
	"fmt"
	"testing"

	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestHttpPermission(t *testing.T) {

	ENTITIES := []Value{
		URL("https://localhost:443/?a=1"),
		URL("https://localhost:443/"),
		URLPattern("https://localhost:443/..."),
		Host("https://localhost:443"),
		HostPattern("https://**"),
	}

	for kind := permkind.Read; kind <= permkind.Provide; kind++ {
		for _, entity := range ENTITIES {
			t.Run(kind.String()+"_"+fmt.Sprint(entity)+"_includes_itself", func(t *testing.T) {
				perm := HttpPermission{Kind_: kind, Entity: entity.(WrappedString)}
				assert.True(t, perm.Includes(perm))
			})
		}
	}

	for kind := permkind.Read; kind <= permkind.Provide; kind++ {
		for i, entity := range ENTITIES {
			for _, prevEntity := range ENTITIES[:i] {
				t.Run(fmt.Sprintf("%s_%s_includes_%s", kind, entity, prevEntity), func(t *testing.T) {
					perm := HttpPermission{Kind_: kind, Entity: entity.(WrappedString)}
					otherPerm := HttpPermission{Kind_: kind, Entity: prevEntity.(WrappedString)}

					assert.True(t, perm.Includes(otherPerm))
				})
			}
		}
	}

	t.Run("a permission with a prefix pattern should include a permission with a longer prefix pattern", func(t *testing.T) {
		perm := HttpPermission{Kind_: permkind.Read, Entity: URLPattern("https://localhost:443/...")}
		otherPerm := HttpPermission{Kind_: permkind.Read, Entity: URL("https://localhost:443/abc/...")}
		assert.True(t, perm.Includes(otherPerm))
	})

	t.Run("schemes should be equal", func(t *testing.T) {
		httpsPerm := HttpPermission{Kind_: permkind.Read, Entity: URLPattern("https://localhost:443/...")}
		httpPerm := HttpPermission{Kind_: permkind.Read, Entity: URL("http://localhost:443/...")}
		assert.False(t, httpsPerm.Includes(httpPerm))
		assert.False(t, httpPerm.Includes(httpsPerm))
	})

	t.Run("write includes create & update", func(t *testing.T) {
		patt := URLPattern("https://localhost:443/...")
		writePerm := HttpPermission{Kind_: permkind.Write, Entity: patt}
		createPerm := HttpPermission{Kind_: permkind.Create, Entity: patt}
		updatePerm := HttpPermission{Kind_: permkind.Update, Entity: patt}

		assert.True(t, writePerm.Includes(createPerm))
		assert.True(t, writePerm.Includes(updatePerm))

		assert.False(t, createPerm.Includes(writePerm))
		assert.False(t, updatePerm.Includes(writePerm))
	})
}

func TestDNSPermission(t *testing.T) {
	testCases := []struct {
		domain1        WrappedString
		domain2        WrappedString
		oneIncludesTwo bool
	}{
		{Host("://a.com"), Host("://a.com"), true},
		{Host("://a.com"), HostPattern("://*.com"), false},
		{HostPattern("://*.com"), HostPattern("://*.com"), true},
		{HostPattern("://*.com"), HostPattern("://**.com"), false},
		{HostPattern("://**.com"), HostPattern("://*.com"), true},
		{HostPattern("://**.com"), HostPattern("://*.org"), false},
		{HostPattern("://**.com"), HostPattern("://*.example.com"), true},
		{HostPattern("://**.com"), HostPattern("://**.example.com"), true},
		{HostPattern("://a.**"), HostPattern("://example.**"), false},
		{HostPattern("://a.**"), HostPattern("://a.*"), true},
		{HostPattern("://a.**"), HostPattern("://a.**"), true},
		{HostPattern("://a.**"), HostPattern("://a.example.**"), true},
	}

	for _, testCase := range testCases {
		FMT := "%s_%s_includes_%s"
		if !testCase.oneIncludesTwo {
			FMT = "%s_%s_does_not_include_%s"
		}

		t.Run(fmt.Sprintf(FMT, t.Name(), testCase.domain1, testCase.domain2), func(t *testing.T) {
			perm1 := DNSPermission{permkind.Read, testCase.domain1}
			perm2 := DNSPermission{permkind.Read, testCase.domain2}

			if testCase.oneIncludesTwo {
				assert.True(t, perm1.Includes(perm2))
			} else {
				assert.False(t, perm1.Includes(perm2))
			}
		})
	}
}

func TestRawTcpPermission(t *testing.T) {
	testCases := []struct {
		domain1        WrappedString
		domain2        WrappedString
		oneIncludesTwo bool
	}{
		{Host("://a.com"), Host("://a.com"), true},
		{Host("://a.com"), HostPattern("://*.com"), false},
		{HostPattern("://*.com"), HostPattern("://*.com"), true},
		{HostPattern("://*.com"), HostPattern("://**.com"), false},
		{HostPattern("://**.com"), HostPattern("://*.com"), true},
		{HostPattern("://**.com"), HostPattern("://*.org"), false},
		{HostPattern("://**.com"), HostPattern("://*.example.com"), true},
		{HostPattern("://**.com"), HostPattern("://**.example.com"), true},
		{HostPattern("://a.**"), HostPattern("://example.**"), false},
		{HostPattern("://a.**"), HostPattern("://a.*"), true},
		{HostPattern("://a.**"), HostPattern("://a.**"), true},
		{HostPattern("://a.**"), HostPattern("://a.example.**"), true},
	}

	for _, testCase := range testCases {
		FMT := "%s_%s_includes_%s"
		if !testCase.oneIncludesTwo {
			FMT = "%s_%s_does_not_include_%s"
		}

		t.Run(fmt.Sprintf(FMT, t.Name(), testCase.domain1, testCase.domain2), func(t *testing.T) {
			perm1 := RawTcpPermission{permkind.Read, testCase.domain1}
			perm2 := RawTcpPermission{permkind.Read, testCase.domain2}

			if testCase.oneIncludesTwo {
				assert.True(t, perm1.Includes(perm2))
			} else {
				assert.False(t, perm1.Includes(perm2))
			}
		})
	}
}

func TestCommandPermission(t *testing.T) {
	permNoSub := CommandPermission{CommandName: Str("mycmd")}
	assert.True(t, permNoSub.Includes(permNoSub))

	permNoSubPath := CommandPermission{CommandName: Path("/bin/env")}
	assert.True(t, permNoSubPath.Includes(permNoSubPath))

	permNoSubPathPattern := CommandPermission{CommandName: PathPattern("/bin/...")}
	assert.True(t, permNoSubPathPattern.Includes(permNoSubPathPattern))
	assert.True(t, permNoSubPathPattern.Includes(permNoSubPath))

	otherPermNoSub := CommandPermission{CommandName: Str("mycmd2")}
	assert.False(t, otherPermNoSub.Includes(permNoSub))
	assert.False(t, permNoSub.Includes(otherPermNoSub))

	permSub1a := CommandPermission{CommandName: Str("mycmd"), SubcommandNameChain: []string{"a"}}
	assert.True(t, permSub1a.Includes(permSub1a))
	assert.False(t, permNoSub.Includes(permSub1a))
	assert.False(t, permSub1a.Includes(permNoSub))

	permSub1b := CommandPermission{CommandName: Str("mycmd"), SubcommandNameChain: []string{"b"}}
	assert.False(t, permSub1b.Includes(permSub1a))
	assert.False(t, permSub1a.Includes(permSub1b))
}

func TestFilesystemPermission(t *testing.T) {
	ENTITIES := []Value{
		Path("./"),
		PathPattern("./..."),
		PathPattern("./*.go"),
	}

	for kind := permkind.Read; kind <= permkind.Provide; kind++ {
		for _, entity := range ENTITIES {
			t.Run(kind.String()+"_"+fmt.Sprint(entity), func(t *testing.T) {
				perm := FilesystemPermission{Kind_: kind, Entity: entity.(WrappedString)}
				assert.True(t, perm.Includes(perm))
			})
		}
	}

	testCases := []struct {
		entity1        WrappedString
		entity2        WrappedString
		oneIncludesTwo bool
	}{
		{PathPattern("/..."), Path("/"), true},
		{PathPattern("/..."), Path("/a"), true},
		{PathPattern("/..."), Path("/a/"), true},
		{PathPattern("/..."), Path("/a/b"), true},
		{PathPattern("/..."), Path("/a/b/"), true},
		{PathPattern("/a/..."), Path("/"), false},
		{PathPattern("/a/..."), Path("/a/"), true},
		{PathPattern("/a/..."), Path("/a/b"), true},
		{PathPattern("/a/..."), Path("/a/b/"), true},
	}

	for _, testCase := range testCases {
		FMT := "%s_%s_includes_%s"
		if !testCase.oneIncludesTwo {
			FMT = "%s_%s_does_not_include_%s"
		}

		t.Run(fmt.Sprintf(FMT, t.Name(), testCase.entity1, testCase.entity2), func(t *testing.T) {
			perm1 := FilesystemPermission{permkind.Read, testCase.entity1}
			perm2 := FilesystemPermission{permkind.Read, testCase.entity2}

			if testCase.oneIncludesTwo {
				assert.True(t, perm1.Includes(perm2))
			} else {
				assert.False(t, perm1.Includes(perm2))
			}
		})
	}

	t.Run("write includes create & update", func(t *testing.T) {
		patt := PathPattern("/...")
		writePerm := FilesystemPermission{Kind_: permkind.Write, Entity: patt}
		createPerm := FilesystemPermission{Kind_: permkind.Create, Entity: patt}
		updatePerm := FilesystemPermission{Kind_: permkind.Update, Entity: patt}

		assert.True(t, writePerm.Includes(createPerm))
		assert.True(t, writePerm.Includes(updatePerm))

		assert.False(t, createPerm.Includes(writePerm))
		assert.False(t, updatePerm.Includes(writePerm))
	})

}

func TestVisibilityPermission(t *testing.T) {
	testCases := []struct {
		pattern        Pattern
		otherPattern   Pattern
		oneIncludesTwo bool
	}{
		{EMAIL_ADDR_PATTERN, EMAIL_ADDR_PATTERN, true},
		{EMAIL_ADDR_PATTERN, NewExactValuePattern(EmailAddress("a@mail.com")), true},
		{EMAIL_ADDR_PATTERN, NewExactValuePattern(Int(0)), false},
	}

	for _, testCase := range testCases {
		FMT := "%s_%s_includes_%s"
		if !testCase.oneIncludesTwo {
			FMT = "%s_%s_does_not_include_%s"
		}

		t.Run(fmt.Sprintf(FMT, t.Name(), testCase.pattern, testCase.otherPattern), func(t *testing.T) {
			perm1 := ValueVisibilityPermission{testCase.pattern}
			perm2 := ValueVisibilityPermission{testCase.otherPattern}

			if testCase.oneIncludesTwo {
				assert.True(t, perm1.Includes(perm2))
			} else {
				assert.False(t, perm1.Includes(perm2))
			}
		})
	}

}

func TestDatabasePermission(t *testing.T) {

	ENTITIES := []Value{
		URL("ldb://main/"),
		Host("ldb://main"),
		HostPattern("ldb://**"),
	}

	for kind := permkind.Read; kind <= permkind.Provide; kind++ {
		for _, entity := range ENTITIES {
			t.Run(kind.String()+"_"+fmt.Sprint(entity)+"_includes_itself", func(t *testing.T) {
				perm := DatabasePermission{Kind_: kind, Entity: entity.(WrappedString)}
				assert.True(t, perm.Includes(perm))
			})
		}
	}

	for kind := permkind.Read; kind <= permkind.Provide; kind++ {
		for i, entity := range ENTITIES {
			for _, prevEntity := range ENTITIES[:i] {
				t.Run(fmt.Sprintf("%s_%s_includes_%s", kind, entity, prevEntity), func(t *testing.T) {
					perm := DatabasePermission{Kind_: kind, Entity: entity.(WrappedString)}
					otherPerm := DatabasePermission{Kind_: kind, Entity: prevEntity.(WrappedString)}

					assert.True(t, perm.Includes(otherPerm))
				})
			}
		}
	}

	t.Run("a permission with a prefix pattern should include a permission with a longer prefix pattern", func(t *testing.T) {
		perm := DatabasePermission{Kind_: permkind.Read, Entity: URLPattern("ldb://main:443/...")}
		otherPerm := DatabasePermission{Kind_: permkind.Read, Entity: URL("ldb://main:443/abc/...")}
		assert.True(t, perm.Includes(otherPerm))
	})

	t.Run("schemes should be equal", func(t *testing.T) {
		ldbPerm := DatabasePermission{Kind_: permkind.Read, Entity: URLPattern("ldb://main:443/...")}
		odbPerm := DatabasePermission{Kind_: permkind.Read, Entity: URL("odb://main:443/...")}
		assert.False(t, ldbPerm.Includes(odbPerm))
		assert.False(t, odbPerm.Includes(ldbPerm))
	})

	t.Run("write includes create & update", func(t *testing.T) {
		patt := URLPattern("ldb://localhost:443/...")
		writePerm := DatabasePermission{Kind_: permkind.Write, Entity: patt}
		createPerm := DatabasePermission{Kind_: permkind.Create, Entity: patt}
		updatePerm := DatabasePermission{Kind_: permkind.Update, Entity: patt}

		assert.True(t, writePerm.Includes(createPerm))
		assert.True(t, writePerm.Includes(updatePerm))

		assert.False(t, createPerm.Includes(writePerm))
		assert.False(t, updatePerm.Includes(writePerm))
	})

	createObjectWithURL := func(perms []Permission) (*Context, *Object) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
			Filesystem:  newMemFilesystem(),
		}, nil)
		db, err := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner: &dummyDatabase{
				resource: Host("ldb://main"),
			},
			Name: "main",
		})
		if !assert.NoError(t, err) {
			return nil, nil
		}
		err = db.SetOwnerStateOnceAndLoadIfNecessary(ctx, ctx.state)
		ctx.state.Databases = map[string]*DatabaseIL{"main": db}

		if !assert.NoError(t, err) {
			return nil, nil
		}
		sharedObject := NewObjectFromMapNoInit(ValMap{
			"a": Int(1),
		})
		err = sharedObject.SetURLOnce(ctx, "ldb://main/object")
		if !assert.NoError(t, err) {
			return nil, nil
		}
		return ctx, sharedObject
	}

	t.Run("missing permission to get object property", func(t *testing.T) {
		ctx, sharedObject := createObjectWithURL(nil)
		if sharedObject == nil {
			return
		}
		func() {
			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}
				err := e.(error)
				assert.ErrorIs(t, err, NewNotAllowedError(DatabasePermission{
					Kind_:  permkind.Read,
					Entity: URL("ldb://main/object/a"),
				}))
			}()
			sharedObject.Prop(ctx, "a")
		}()
	})

	t.Run("missing permission to set object property", func(t *testing.T) {
		ctx, sharedObject := createObjectWithURL(nil)
		if sharedObject == nil {
			return
		}
		func() {
			err := sharedObject.SetProp(ctx, "a", Int(2))
			assert.ErrorIs(t, err, NewNotAllowedError(DatabasePermission{
				Kind_:  permkind.Write,
				Entity: URL("ldb://main/object/a"),
			}))
		}()
	})

	t.Run("missing permission to get top-level entity of database", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                        db,
			OwnerState:                   ctx.state,
			ForceLoadBeforeOwnerStateSet: true,
		}))

		func() {
			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}
				err := e.(error)
				assert.ErrorIs(t, err, NewNotAllowedError(DatabasePermission{
					Kind_:  permkind.Read,
					Entity: URL("ldb://main/a"),
				}))
			}()
			dbIL.Prop(ctx, "a")
		}()
	})

	t.Run("granted permission to get top-level entity of database", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                        db,
			OwnerState:                   ctx.state,
			ForceLoadBeforeOwnerStateSet: true,
		}))

		assert.NotPanics(t, func() {
			dbIL.Prop(ctx, "a")
		})
	})
}
