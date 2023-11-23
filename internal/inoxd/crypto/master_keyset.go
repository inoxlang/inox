package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/inoxlang/inox/internal/project/systemdprovider/unitenv"
	"github.com/tink-crypto/tink-go/aead"
	"github.com/tink-crypto/tink-go/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/keyset"
)

var (
	INOXD_MASTER_KEYSET_ENCODING = base64.StdEncoding

	ErrInoxdMasterKeysetEnvVarNotFound    = errors.New(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME + " is not set")
	ErrInoxdMasterKeysetEnvVarSetButEmpty = errors.New(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME + " is set but is empty")
)

type JSONSerializedKeySet string

func InoxdMasterKeySetHandleFrom(serializedKeyset string) (*keyset.Handle, error) {
	return insecurecleartextkeyset.Read(
		keyset.NewJSONReader(bytes.NewBuffer([]byte(serializedKeyset))))
}

func LoadInoxdMasterKeysetFromEnv() (*keyset.Handle, error) {
	value, ok := os.LookupEnv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME)
	if !ok {
		return nil, fmt.Errorf("inoxd master keyset not found in environment variables: %w", ErrInoxdMasterKeysetEnvVarNotFound)
	}
	if value == "" {
		return nil, fmt.Errorf("inoxd master keyset not found in environment variables: %w", ErrInoxdMasterKeysetEnvVarSetButEmpty)
	}

	os.Unsetenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME)

	return InoxdMasterKeySetHandleFrom(value)
}

func GenerateRandomInoxdMasterKeyset() JSONSerializedKeySet {
	// Generate a new keyset handle for the primitive we want to use.
	handle, err := keyset.NewHandle(aead.AES256GCMKeyTemplate())
	if err != nil {
		log.Fatal(err)
	}

	// Serialize the keyset.
	buff := &bytes.Buffer{}
	err = insecurecleartextkeyset.Write(handle, keyset.NewJSONWriter(buff))
	if err != nil {
		log.Fatal(err)
	}
	serializedKeyset := buff.Bytes()
	return JSONSerializedKeySet(serializedKeyset)
}
