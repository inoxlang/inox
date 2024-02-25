package projectserver

import (
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type CommitInfo struct {
	HashHex   string              `json:"hashHex"`
	Message   string              `json:"message"`
	Author    CommitSignatureInfo `json:"author"`
	Committer CommitSignatureInfo `json:"committer"`
}

type CommitSignatureInfo struct {
	// Name represents a person name. It is an arbitrary string.
	Name string `json:"name"`
	// Email is an email, but it cannot be assumed to be well-formed.
	Email string `json:"email"`
	// When is the timestamp of the signature.
	When time.Time `json:"when"`
}

func makeCommitInfo(commit *object.Commit) CommitInfo {
	return CommitInfo{
		HashHex:   commit.Hash.String(),
		Message:   commit.Message,
		Author:    CommitSignatureInfo(commit.Author),
		Committer: CommitSignatureInfo(commit.Committer),
	}
}
