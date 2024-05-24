package core

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/oklog/ulid/v2"
)

// This file contain several Id types.

var (
	_ = []Serializable{ULID{}, UUIDv4{}}

	cryptoSecureMonotonicEntropySrcForULIDs = &ulid.LockedMonotonicReader{
		MonotonicReader: ulid.Monotonic(CryptoRandSource, math.MaxUint32),
	}

	MIN_ULID, MAX_ULID ULID
)

func init() {
	max := (*ulid.ULID)(&MAX_ULID)
	max.SetTime(ulid.MaxTime())
	entropy := [10]byte{}

	for i := 0; i < len(entropy); i++ {
		entropy[i] = 255
	}
	max.SetEntropy(entropy[:])
}

// ULID implements Value.
type ULID ulid.ULID

// NewULID generates in a cryptographically secure way an ULID with a monotonically increasing entropy.
func NewULID() ULID {
	id := ulid.MustNew(ulid.Now(), cryptoSecureMonotonicEntropySrcForULIDs)
	return ULID(id)
}

func ParseULID(s string) (ULID, error) {
	id, err := ulid.ParseStrict(s)
	if err != nil {
		return ULID{}, err
	}
	return ULID(id), nil
}

func (id ULID) String() string {
	return id.libValue().String()
}

func (id ULID) libValue() ulid.ULID {
	return ulid.ULID(id)
}

func (id ULID) GoTime() time.Time {
	return time.UnixMilli(int64(id.libValue().Time()))
}

func (id ULID) Time() DateTime {
	return DateTime(id.GoTime())
}

func (id ULID) After(other ULID) bool {
	result, _ := id.Compare(other)
	return result == 1
}

func (id ULID) Before(other ULID) bool {
	result, _ := id.Compare(other)
	return result == -1
}

// UUIDv4 implements Value.
type UUIDv4 uuid.UUID

func NewUUIDv4() UUIDv4 {
	value := utils.Must(uuid.NewRandom())
	return UUIDv4(value)
}

func ParseUUIDv4(s string) (UUIDv4, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UUIDv4{}, err
	}
	if id.Version() != 4 {
		return UUIDv4{}, errors.New("UUID version it not 4")
	}
	return UUIDv4(id), nil
}

func (id UUIDv4) libValue() uuid.UUID {
	return uuid.UUID(id)
}

func (id UUIDv4) String() string {
	return id.libValue().String()
}
