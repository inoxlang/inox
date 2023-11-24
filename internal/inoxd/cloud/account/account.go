package account

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/alexedwards/argon2id"
	"github.com/oklog/ulid/v2"
)

const (
	DISPOSABLE_TOKEN_ID_LENGTH = 32
)

var (
	INFO_HASH_PARAMS = argon2id.Params{
		Memory:      64 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  32,
		KeyLength:   16,
	}
)

type AnonymousAccount struct {
	ULID string `json:"id"`

	//hex-encoded argon2id hash of concatenated information, the salt and the hashing parameters are stored in the string.
	InformationHash string `json:"informationHash"`

	//a hex-encoded sha256 hash is used because the token is a random string and we want the hashing to be very fast.
	TokenHash TokenHash `json:"tokenHash"`
}

type DisposableAccountCreation struct {
	Hoster           ProofHoster
	UserIdOnHoster   string
	UsernameOnHoster string
}

func NewDisposableAccount(input DisposableAccountCreation) (account *AnonymousAccount, hexEncodedToken string, _ error) {
	//create and hash a truly random token
	token := [DISPOSABLE_TOKEN_ID_LENGTH]byte{}
	_, err := rand.Read(token[:])
	if err != nil {
		return nil, "", err
	}

	hexEncodedToken = hex.EncodeToString(token[:])
	tokenHash, err := HashCleartextToken(hexEncodedToken)
	if err != nil {
		return nil, "", err
	}

	//compute the information hash
	concatenatedInfo := input.UserIdOnHoster + "$" + input.UsernameOnHoster
	infoHash, err := argon2id.CreateHash(concatenatedInfo, &INFO_HASH_PARAMS)
	if err != nil {
		return nil, "", err
	}

	return &AnonymousAccount{
		ULID:            ulid.Make().String(),
		TokenHash:       tokenHash,
		InformationHash: infoHash,
	}, hexEncodedToken, nil
}

type TokenHash string

func HashCleartextToken(token string) (TokenHash, error) {
	tokenBytes, err := hex.DecodeString(token)
	if err != nil {
		return "", err
	}
	hashBytes := sha256.Sum256(tokenBytes)
	return TokenHash(hex.EncodeToString(hashBytes[:])), nil
}
