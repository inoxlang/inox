package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	_ = SerializedValueStorage((*TestValueStorage)(nil))
)

func TestLoadObject(t *testing.T) {

	t.Run("non existing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
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
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
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
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{"a":"1"}`},
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

		assert.Equal(t, `{"_url_":"ldb://main/user","a":"2"}`, storage.Data["/user"])
	})

	t.Run("migration: deletion", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
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
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
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
		assert.Equal(t, `{"_url_":"ldb://main/user","a":"1"}`, storage.Data["/user"])
	})

	t.Run("migration: new property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		storage := &TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[Path]string{"/user": `{"a":"1"}`},
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
		assert.Equal(t, `{"_url_":"ldb://main/user","a":"1","b":"2"}`, storage.Data["/user"])
	})

	t.Run("migration: new property + allow missing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
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
