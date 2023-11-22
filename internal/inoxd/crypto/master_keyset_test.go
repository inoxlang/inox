package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tink-crypto/tink-go/aead"
)

func TestInoxdMasterKeySet(t *testing.T) {
	key := GenerateRandomInoxdMasterKeySet()

	handle, err := InoxdMasterKeySetHandleFrom(string(key))
	if !assert.NoError(t, err) {
		return
	}

	primitive, err := aead.New(handle)
	if !assert.NoError(t, err) {
		return
	}

	plaintext := []byte("message")
	associatedData := []byte("example encryption")
	ciphertext, err := primitive.Encrypt(plaintext, associatedData)
	if !assert.NoError(t, err) {
		return
	}

	decrypted, err := primitive.Decrypt(ciphertext, associatedData)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "message", string(decrypted))
}
