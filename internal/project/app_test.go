package project

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestRegisterApplication(t *testing.T) {
	const APP_NAME = "myapp"

	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	fls := fs_ns.NewMemFilesystem(1_000)

	reg := utils.Must(OpenRegistry("/projects", fls, ctx))
	defer reg.Close(ctx)

	id, err := reg.CreateProject(ctx, CreateProjectParams{
		Name: "myproject",
	})

	if !assert.NoError(t, err) {
		return
	}

	project, err := reg.OpenProject(ctx, OpenProjectParams{
		Id: id,
	})

	if !assert.NoError(t, err) {
		return
	}

	err = project.RegisterApplication(ctx, APP_NAME)

	if !assert.NoError(t, err) {
		return
	}

	//check the application is registered

	appNames := project.ApplicationNames(ctx)
	assert.EqualValues(t, []node.ApplicationName{APP_NAME}, appNames)

	//reopen the projet and check again

	reg.Close(ctx)
	reg = utils.Must(OpenRegistry("/projects", fls, ctx))

	project, err = reg.OpenProject(ctx, OpenProjectParams{
		Id: id,
	})

	if !assert.NoError(t, err) {
		return
	}

	appNames = project.ApplicationNames(ctx)
	assert.EqualValues(t, []node.ApplicationName{APP_NAME}, appNames)
}
