package crypto

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tink-crypto/tink-go/aead"
)

func TestInoxdMasterKeySet(t *testing.T) {
	key := GenerateRandomInoxdMasterKeyset()

	handle, err := InoxdMasterKeySetHandleFrom(string(key))
	if !assert.NoError(t, err) {
		return
	}

	primitive, err := aead.New(handle)
	if !assert.NoError(t, err) {
		return
	}

	//check the keyset by encrypting + decrypting.

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

func TestLoadInoxdMasterKeysetFromEnv(t *testing.T) {
	save, ok := os.LookupEnv(INOXD_MASTER_KEYSET_ENV_VARNAME)
	if ok {
		defer os.Setenv(INOXD_MASTER_KEYSET_ENV_VARNAME, save)
	} else {
		defer os.Unsetenv(INOXD_MASTER_KEYSET_ENV_VARNAME)
	}

	key := GenerateRandomInoxdMasterKeyset()
	os.Setenv(INOXD_MASTER_KEYSET_ENV_VARNAME, string(key))

	keyset, err := LoadInoxdMasterKeysetFromEnv()
	if !assert.NoError(t, err) {
		return
	}

	primitive, err := aead.New(keyset)
	if !assert.NoError(t, err) {
		return
	}

	//check the keyset by encrypting + decrypting.

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
