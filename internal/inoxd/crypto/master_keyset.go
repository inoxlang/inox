package crypto

import (
	"bytes"
	"encoding/base64"
	"log"

	"github.com/tink-crypto/tink-go/aead"
	"github.com/tink-crypto/tink-go/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/keyset"
)

var (
	INOXD_MASTER_KEY_ENCODING = base64.StdEncoding
)

type JSONSerializedKeySet string

func InoxdMasterKeySetHandleFrom(serializedKeyset string) (*keyset.Handle, error) {
	return insecurecleartextkeyset.Read(
		keyset.NewJSONReader(bytes.NewBuffer([]byte(serializedKeyset))))
}

func GenerateRandomInoxdMasterKeySet() JSONSerializedKeySet {
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
