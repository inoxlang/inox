package project

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/access"
)

var (
	ErrFailedMemberAuthentication = errors.New("failed member authentication")
)

func (project *Project) AuthenticateMember(ctx *core.Context, id access.MemberID /*token string*/) (*access.Member, error) {
	member, ok := project.GetMemberByID(ctx, id)
	if !ok {
		return nil, ErrFailedMemberAuthentication
	}

	//TODO: implement authentication

	return member, nil
}
