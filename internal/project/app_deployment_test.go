package project_test

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/inoxd/nodeimpl"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	_ "github.com/inoxlang/inox/internal/globals"
)

func TestSameProcessDeployment(t *testing.T) {
	const APP_NAME = "myapp"
	host := "https://localhost:8080"

	//create node agent

	{
		prodDir := t.TempDir()

		goCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		agent, err := nodeimpl.NewAgent(nodeimpl.AgentParameters{
			GoCtx:  goCtx,
			Logger: zerolog.Nop(),
			Config: nodeimpl.AgentConfig{
				OsProdDir:                       core.DirPathFrom(prodDir),
				TemporaryOptionRunInSameProcess: true,
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		node.SetAgent(agent)
	}

	// create project

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN},
		},
		Filesystem: fs_ns.GetOsFilesystem(),
	}, nil)
	defer ctx.CancelGracefully()

	reg := utils.Must(project.OpenRegistry(t.TempDir(), ctx))
	defer reg.Close(ctx)

	id, _, err := reg.CreateProject(ctx, project.CreateProjectParams{
		Name: "myproject",
	})

	if !assert.NoError(t, err) {
		return
	}

	proj, err := reg.OpenProject(ctx, project.OpenProjectParams{
		Id: id,
	})

	if !assert.NoError(t, err) {
		return
	}

	//create application module in the project's filesystem

	modPath := "/main.ix"

	util.WriteFile(proj.LiveFilesystem(), modPath, []byte(`
		manifest {
			kind:"application"
			permissions: {
				provide: `+host+`
			}
		}

		# server answering with hello
		server = http.Server!(`+host+`)
		server.wait_closed()
	`), 0600)

	// preparing the deployment without having registered the app is not allowed

	_, err = proj.PrepareApplicationDeployment(ctx, project.ApplicationDeploymentPreparationParams{
		AppName:          APP_NAME,
		UpdateRunningApp: false,
	})

	if !assert.ErrorIs(t, err, project.ErrAppNotRegistered) {
		return
	}

	//register the application

	err = proj.RegisterApplication(ctx, APP_NAME, modPath)

	if !assert.NoError(t, err) {
		return
	}

	// prepare the deployment and deploy

	deployment, err := proj.PrepareApplicationDeployment(ctx, project.ApplicationDeploymentPreparationParams{
		AppName:          APP_NAME,
		UpdateRunningApp: false,
	})

	if !assert.NoError(t, err) {
		return
	}

	err = deployment.Perform()

	if !assert.NoError(t, err) {
		return
	}

	// check the status of the deployment and the application

	assert.Equal(t, node.SuccessfulDeployment, deployment.Status())
	assert.Equal(t, map[node.ApplicationName]node.ApplicationStatus{APP_NAME: node.DeployedApp}, proj.ApplicationStatuses(ctx))
	assert.Equal(t, map[node.ApplicationName]string{APP_NAME: node.DeployedApp.String()}, proj.ApplicationStatusNames(ctx))

	app, ok := node.GetAgent().GetApplication(APP_NAME)
	if !assert.True(t, ok) {
		return
	}
	defer app.UnsafelyStop()

	assert.Equal(t, node.DeployedApp, app.Status())

	// make an HTTP request to check the application is running

	client := *http.DefaultClient
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	resp, err := client.Get(host + "/")
	if resp != nil {
		defer resp.Body.Close()
	}

	if !assert.NoError(t, err) {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "hello", string(body))
}
