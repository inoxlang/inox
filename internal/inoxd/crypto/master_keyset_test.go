package crypto

import (
	"os"
	"testing"

	"github.com/inoxlang/inox/internal/inoxd/systemd/unitenv"
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
	varname := unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME

	t.Run("if the env var is not set an error should be returned", func(t *testing.T) {
		save, ok := os.LookupEnv(varname)
		if ok {
			defer os.Setenv(varname, save)
		} else {
			defer os.Unsetenv(varname)
		}

		_, err := LoadInoxdMasterKeysetFromEnv()
		assert.ErrorIs(t, err, ErrInoxdMasterKeysetEnvVarNotFound)
	})

	t.Run("if the env var is set but empty an error should be returned", func(t *testing.T) {
		save, ok := os.LookupEnv(varname)
		if ok {
			defer os.Setenv(varname, save)
		} else {
			defer os.Unsetenv(varname)
		}

		os.Setenv(varname, "")

		_, err := LoadInoxdMasterKeysetFromEnv()
		assert.ErrorIs(t, err, ErrInoxdMasterKeysetEnvVarSetButEmpty)
	})

	t.Run("env var is set", func(t *testing.T) {
		save, ok := os.LookupEnv(varname)
		if ok {
			defer os.Setenv(varname, save)
		} else {
			defer os.Unsetenv(varname)
		}

		key := GenerateRandomInoxdMasterKeyset()
		os.Setenv(varname, string(key))

		keyset, err := LoadInoxdMasterKeysetFromEnv()
		if !assert.NoError(t, err) {
			return
		}

		_, ok = os.LookupEnv(varname)
		assert.False(t, ok)

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
	})
}
