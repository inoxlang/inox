package access

import "errors"

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

func (m Member) ID() string {
	return m.data.ID
}

// Persisted member data.
type MemberData struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

func (data MemberData) Validate() error {
	if data.Name == "" {
		return errors.New("invalid member data: empty name")
	}
	if data.ID == "" {
		return errors.New("invalid member data: empty id")
	}
	return nil
}
