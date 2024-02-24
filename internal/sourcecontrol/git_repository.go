package sourcecontrol

import (
	"sync"

	"github.com/go-git/go-git/v5"
)

type GitRepository struct {
	lock  sync.RWMutex
	inner *git.Repository //repository on project server
}

func WrapLower(repo *git.Repository) *GitRepository {
	return &GitRepository{
		inner: repo,
	}
}
