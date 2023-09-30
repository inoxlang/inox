package core

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/stretchr/testify/assert"
)

func TestSecrets(t *testing.T) {

	t.Run("printing a secret should not leak its value", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		buf := bytes.NewBuffer(nil)
		fmt.Fprint(buf, secret)
		assert.NotContains(t, buf.String(), secretValue)

		buf = bytes.NewBuffer(nil)
		fmt.Fprintf(buf, "%s", secret)
		assert.NotContains(t, buf.String(), secretValue)

		buf = bytes.NewBuffer(nil)
		fmt.Fprintf(buf, "%q", secret)
		assert.NotContains(t, buf.String(), secretValue)

		buf = bytes.NewBuffer(nil)
		fmt.Fprintf(buf, "%v", secret)
		assert.NotContains(t, buf.String(), secretValue)

		buf = bytes.NewBuffer(nil)
		fmt.Fprintf(buf, "%#v", secret)
		assert.NotContains(t, buf.String(), secretValue)

		buf = bytes.NewBuffer(nil)
		fmt.Fprintf(buf, "%d", secret)
		assert.NotContains(t, buf.String(), secretValue)

	})

	t.Run("serializing a secret to JSON should return an error", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = secret.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0)
			if !assert.ErrorIs(t, err, ErrNoRepresentation) {
				return
			}
			stream.Flush()
			assert.Empty(t, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = secret.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: true},
			}, 0)
			if !assert.ErrorIs(t, err, ErrNoRepresentation) {
				return
			}
			stream.Flush()
			assert.Empty(t, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = secret.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: false},
			}, 0)
			if !assert.ErrorIs(t, err, ErrNoRepresentation) {
				return
			}
			stream.Flush()
			assert.Empty(t, buf.String())
		}
	})

	t.Run("serializing a secret to IXON should return an error", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = secret.WriteRepresentation(ctx, stream, nil, 0)
			if !assert.ErrorIs(t, err, ErrNoRepresentation) {
				return
			}
			stream.Flush()
			assert.Empty(t, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = secret.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: true}, 0)
			if !assert.ErrorIs(t, err, ErrNoRepresentation) {
				return
			}
			stream.Flush()
			assert.Empty(t, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = secret.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: false}, 0)
			if !assert.ErrorIs(t, err, ErrNoRepresentation) {
				return
			}
			stream.Flush()
			assert.Empty(t, buf.String())
		}
	})

	t.Run("serializing to JSON an object only containing a secret should not leak its value", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		object := NewObjectFromMapNoInit(ValMap{
			"a": secret,
		})

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"object__value":{}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				Pattern: OBJECT_PATTERN,
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: true},
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"object__value":{}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: true},
				Pattern:    OBJECT_PATTERN,
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: false},
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"object__value":{}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: false},
				Pattern:    OBJECT_PATTERN,
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, "{}", buf.String())
		}
	})

	t.Run("serializing to JSON an object containing a secret & a visible property should not leak its value", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		object := NewObjectFromMapNoInit(ValMap{
			"a": secret,
			"b": Int(1),
		})

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"object__value":{"b":{"int__value":1}}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				Pattern: OBJECT_PATTERN,
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"b":{"int__value":1}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: true},
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"object__value":{"b":{"int__value":1}}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: true},
				Pattern:    OBJECT_PATTERN,
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"b":{"int__value":1}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: false},
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"object__value":{"b":{"int__value":1}}}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
				ReprConfig: &ReprConfig{AllVisible: false},
				Pattern:    OBJECT_PATTERN,
			}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"b":{"int__value":1}}`, buf.String())
		}
	})

	t.Run("serializing to IXON an object only containing a secret should not leak its value", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		object := NewObjectFromMapNoInit(ValMap{
			"a": secret,
		})

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: true}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: true}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: false}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{}`, buf.String())
		}
	})

	t.Run("serializing to IXON an object containing a secret & a visible property should not leak its value", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		const secretValue = "mysecret"
		secret, err := SECRET_STRING_PATTERN.NewSecret(ctx, secretValue)
		if !assert.NoError(t, err) {
			return
		}

		object := NewObjectFromMapNoInit(ValMap{
			"a": secret,
			"b": Int(1),
		})

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: true}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"b":1}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: true}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"b":1}`, buf.String())
		}

		{
			buf := bytes.NewBuffer(nil)
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 10)
			err = object.WriteRepresentation(ctx, stream, &ReprConfig{AllVisible: false}, 0)
			if !assert.NoError(t, err) {
				return
			}
			stream.Flush()
			assert.Equal(t, `{"b":1}`, buf.String())
		}
	})
}
