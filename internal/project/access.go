package project

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/access"
)

func (project *Project) AuthenticateMember(ctx *core.Context, id access.MemberID /*token string*/) (*access.Member, bool) {
	member, ok := project.GetMemberByID(ctx, id)
	if !ok {
		return nil, false
	}

	//TODO: implement authentication

	return member, true
}
