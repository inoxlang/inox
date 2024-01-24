package core

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

// This file contain several Id types.

var (
	_ = []Serializable{ULID{}, UUIDv4{}}
)

// ULID implements Value.
type ULID ulid.ULID

func NewULID() ULID {
	return ULID(ulid.Make())
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

func (id ULID) Time() time.Time {
	return time.UnixMilli(int64(id.libValue().Time()))
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
