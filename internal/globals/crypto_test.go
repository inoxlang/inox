package globals

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestRSAGenKey(t *testing.T) {

	ctx := core.NewContext(core.ContextConfig{})

	assert.NotPanics(t, func() {
		record := _rsa_gen_key(ctx)
		assert.True(t, record.HasProp(ctx, KEY_PAIR_RECORD_PROPNAMES[0]))
		assert.True(t, record.HasProp(ctx, KEY_PAIR_RECORD_PROPNAMES[1]))
	})
}

func TestRSAEncryptDecryptOAEP(t *testing.T) {
	ctx := core.NewContext(core.ContextConfig{})

	record := _rsa_gen_key(ctx)
	public := record.Prop(ctx, "public").(core.StringLike)
	private := record.Prop(ctx, "private").(*core.Secret)

	encrypted, err := _rsa_encrypt_oaep(ctx, core.String("hello"), public)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotZero(t, encrypted.Len()) {
		return
	}

	decrypted, err := _rsa_decrypt_oaep(ctx, encrypted, private)
	assert.NoError(t, err)
	assert.Equal(t, decrypted.UnderlyingBytes(), []byte("hello"))
}
