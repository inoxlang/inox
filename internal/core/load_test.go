package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

var (
	_ = SerializedValueStorage((*TestValueStorage)(nil))
)

func TestLoadObject(t *testing.T) {

	perms := []Permission{
		DatabasePermission{
			Kind_:  permkind.Read,
			Entity: URLPattern("ldb://main/..."),
		},
		DatabasePermission{
			Kind_:  permkind.Write,
			Entity: URLPattern("ldb://main/..."),
		},
	}

	t.Run("non existing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{BaseURL_: "ldb://main"}
		pattern := NewInexactObjectPattern(map[string]Pattern{})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration:    nil,
		})

		if !assert.ErrorIs(t, err, ErrFailedToLoadNonExistingValue) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("allow missing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{BaseURL_: "ldb://main"}
		pattern := NewInexactObjectPattern(map[string]Pattern{})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: true,
			Migration:    nil,
		})

		if !assert.ErrorIs(t, err, ErrFailedToLoadNonExistingValue) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("existing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{"a":1}`},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration:    nil,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, val) {
			return
		}
		object := val.(*Object)

		url, _ := object.URL()

		if !assert.Equal(t, URL("ldb://main/user"), url) {
			return
		}

		assert.True(t, object.IsShared())

		//any change to the object should cause a save
		if !assert.NoError(t, object.SetProp(ctx, "a", Int(2))) {
			return
		}

		assert.Equal(t, `{"_url_":"ldb://main/user","a":2}`, storage.Data["/user"])
	})

	t.Run("performing a mutation on a property with a sharable value should cause a save", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{"inner":{"a": 1}}`},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{"inner": NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration:    nil,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, val) {
			return
		}
		object := val.(*Object)

		url, _ := object.URL()

		if !assert.Equal(t, URL("ldb://main/user"), url) {
			return
		}

		assert.True(t, object.IsShared())

		//any deep change to the object should cause a save
		inner := object.Prop(ctx, "inner").(*Object)
		if !assert.NoError(t, inner.SetProp(ctx, "a", Int(2))) {
			return
		}

		assert.Equal(t, `{"_url_":"ldb://main/user","inner":{"a":2}}`, storage.Data["/user"])
	})

	t.Run("performing a mutation on a property with a mutable non-sharable value should cause a save", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		StartNewTransaction(ctx)

		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{"inner":[]}`},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{
			"inner": NewListPatternOf(INT_PATTERN),
		})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration:    nil,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, val) {
			return
		}
		object := val.(*Object)

		url, _ := object.URL()

		if !assert.Equal(t, URL("ldb://main/user"), url) {
			return
		}

		assert.True(t, object.IsShared())

		//any deep change to the object should cause a save
		inner := object.PropNotStored(ctx, "inner").(*List)
		inner.append(ctx, Int(1))

		assert.Equal(t, `{"_url_":"ldb://main/user","inner":[1]}`, storage.Data["/user"])
	})

	t.Run("migration: deletion", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{}`},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration: &InstanceMigrationArgs{
				MigrationHandlers: MigrationOpHandlers{
					Deletions: map[PathPattern]*MigrationOpHandler{
						"/user": nil,
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Nil(t, val)
	})

	t.Run("migration: replacement", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{}`},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{})
		nextPattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration: &InstanceMigrationArgs{
				NextPattern: nextPattern,
				MigrationHandlers: MigrationOpHandlers{
					Replacements: map[PathPattern]*MigrationOpHandler{
						"/user": {
							InitialValue: NewObjectFromMap(ValMap{"a": Int(1)}, ctx),
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, val) {
			return
		}
		object := val.(*Object)

		url, _ := object.URL()

		if !assert.Equal(t, URL("ldb://main/user"), url) {
			return
		}

		assert.True(t, object.IsShared())

		//make sure the post-migration value is saveds
		assert.Equal(t, `{"_url_":"ldb://main/user","a":1}`, storage.Data["/user"])
	})

	t.Run("migration: new property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{"a":1}`},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		nextPattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN, "b": INT_PATTERN})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration: &InstanceMigrationArgs{
				NextPattern: nextPattern,
				MigrationHandlers: MigrationOpHandlers{
					Inclusions: map[PathPattern]*MigrationOpHandler{
						"/user/b": {
							InitialValue: Int(2),
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, val) {
			return
		}
		object := val.(*Object)

		url, _ := object.URL()

		if !assert.Equal(t, URL("ldb://main/user"), url) {
			return
		}

		assert.True(t, object.IsShared())

		//make sure the post-migration value is saveds
		assert.Equal(t, `{"_url_":"ldb://main/user","a":1,"b":2}`, storage.Data["/user"])
	})

	t.Run("migration: new property + allow missing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: perms,
		}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{},
		}
		pattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		nextPattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN, "b": INT_PATTERN})

		val, err := loadObject(ctx, InstanceLoadArgs{
			Key:          "/user",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: true,
			Migration: &InstanceMigrationArgs{
				NextPattern: nextPattern,
				MigrationHandlers: MigrationOpHandlers{
					Inclusions: map[PathPattern]*MigrationOpHandler{
						"/user/b": {
							InitialValue: Int(2),
						},
					},
				},
			},
		})

		if !assert.ErrorIs(t, err, ErrFailedToLoadNonExistingValue) {
			return
		}
		assert.Nil(t, val)
	})
}
