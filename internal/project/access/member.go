package access

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
)

type Member struct {
	data MemberData
}

func MemberFromData(data MemberData) (*Member, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return &Member{
		data: data,
	}, nil
}

func (m Member) Name() string {
	return m.data.Name
}

func (m Member) ID() MemberID {
	return m.data.ID
}

// Persisted member data.
type MemberData struct {
	Name string   `json:"name"`
	ID   MemberID `json:"id"`
}

type MemberID string //uuidv4

func RandomMemberID() MemberID {
	return MemberID(core.NewUUIDv4().String())
}

func (id MemberID) Validate() error {
	if id == "" {
		return errors.New("empty member id")
	}

	_, err := core.ParseUUIDv4(string(id))
	if err != nil {
		return err
	}
	return nil
}

func (data MemberData) Validate() error {
	if data.Name == "" {
		return errors.New("invalid member data: empty name")
	}
	if err := data.ID.Validate(); err != nil {
		return fmt.Errorf("invalid member data: invalid id: %w", err)
	}
	return nil
}
