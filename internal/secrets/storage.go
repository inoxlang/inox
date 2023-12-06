package secrets

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
)

var (
	ErrFailedToListSecrets = errors.New("failed to list secrets")
)

type SecretStorage interface {
	UpsertSecret(ctx *core.Context, name, value string) error
	ListSecrets(ctx *core.Context) (info []core.ProjectSecretInfo, _ error)
	GetSecrets(ctx *core.Context) (secrets []core.ProjectSecret, _ error)
	DeleteSecret(ctx *core.Context, name string) error
}
