package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	_ = SerializedValueStorage((*testValueStorage)(nil))
)

func TestLoadObject(t *testing.T) {

	t.Run("non existing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		storage := &testValueStorage{baseURL: "ldb://main"}
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
		storage := &testValueStorage{baseURL: "ldb://main"}
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
		storage := &testValueStorage{
			baseURL: "ldb://main/",
			data:    map[Path]string{"/user": `{"a":"1"}`},
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

		assert.Equal(t, `{"_url_":"ldb://main/user","a":"2"}`, storage.data["/user"])
	})

	t.Run("migration: new property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		storage := &testValueStorage{
			baseURL: "ldb://main/",
			data:    map[Path]string{"/user": `{"a":"1"}`},
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
		assert.Equal(t, `{"_url_":"ldb://main/user","a":"1","b":"2"}`, storage.data["/user"])
	})

	t.Run("migration: new property + allow missing", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		storage := &testValueStorage{
			baseURL: "ldb://main/",
			data:    map[Path]string{},
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

type testValueStorage struct {
	baseURL URL
	data    map[Path]string
}

func (s *testValueStorage) BaseURL() URL {
	return s.baseURL
}

func (s *testValueStorage) GetSerialized(ctx *Context, key Path) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}

func (s *testValueStorage) Has(ctx *Context, key Path) bool {
	_, ok := s.data[key]
	return ok
}

func (s *testValueStorage) InsertSerialized(ctx *Context, key Path, serialized string) {
	_, ok := s.data[key]
	if !ok {
		panic(errors.New("already present"))
	}
	s.data[key] = serialized
}

func (s *testValueStorage) SetSerialized(ctx *Context, key Path, serialized string) {
	s.data[key] = serialized
}
