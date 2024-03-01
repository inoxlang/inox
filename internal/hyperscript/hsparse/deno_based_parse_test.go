package hsparse

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	_ "embed"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/deno"
	"github.com/inoxlang/inox/internal/deno/binary"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

var (
	DENO_BINARY_USED_FOR_EXEC_TESTS = "/tmp/deno-test"
)

func TestParseHyperscriptWithDeno(t *testing.T) {

	//Install Deno.

	binaryLocation := DENO_BINARY_USED_FOR_EXEC_TESTS

	err := binary.Install(binaryLocation)

	if !assert.NoError(t, err) {
		return
	}

	//Setup the control server.

	perms := []core.Permission{
		core.WebsocketPermission{Kind_: permkind.Provide},
		core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
	}
	limits := []core.Limit{
		{Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},
		{Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},

		{Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 100 * core.FREQ_LIMIT_SCALE},
		{Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.TotalLimit, Value: 10_000},
	}

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: perms,
		Limits:      limits,
	}, nil)
	defer ctx.CancelGracefully()

	go func() {
		<-time.After(10 * time.Second)
		ctx.CancelGracefully()
	}()

	controlServerCtx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: perms,
		Filesystem:  fs_ns.GetOsFilesystem(),
		Limits:      ctx.Limits(),
	}, nil)

	controlServer, err := deno.NewControlServer(controlServerCtx, deno.ControlServerConfig{
		Port: 61_000,
	})

	if !assert.NoError(t, err) {
		return
	}

	//Start the service.

	earlyErrChan := make(chan error)
	go func() {
		defer utils.Recover()
		earlyErrChan <- controlServer.Start()
	}()

	select {
	case err = <-earlyErrChan:
		if !assert.NoError(t, err) {
			return
		}
	case <-time.After(100 * time.Millisecond):
	}

	startService := func(program string) (ulid.ULID, error) {
		id, err := controlServer.StartServiceProcess(controlServerCtx, deno.ServiceConfiguration{
			RequiresPersistendWorkdir: false,
			Name:                      "hyperscript-parser",
			DenoBinaryLocation:        binaryLocation,
			ServiceProgram:            DENO_SERVICE_TS,
			AllowNetwork:              true,
			AllowLocalhostAccess:      true,
		})

		if !assert.NoError(t, err) {
			return ulid.ULID{}, err
		}
		return id, nil
	}

	err = StartHyperscriptParsingService(startService, func(ctx context.Context, input string, serviceID ulid.ULID) (json.RawMessage, error) {
		process, ok := controlServer.GetServiceProcessByID(serviceID)
		if !assert.True(t, ok) {
			return nil, errors.New("service not found")
		}

		return process.CallMethod(controlServerCtx, "parseHyperScript", map[string]any{"input": input, "doNotIncludeNodeData": true})
	})

	if !assert.NoError(t, err) {
		return
	}

	//Wait for the service to be started.
	time.Sleep(100 * time.Millisecond)

	//First try. Should be < 10ms.
	code := strings.Repeat("on click toggle .red on me\nend\n", 10)

	startTime := time.Now()
	result, parsingErr, err := tryParseHyperScriptWithDenoService(ctx, code)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Nil(t, parsingErr) {
		return
	}

	assert.NotNil(t, result.NodeData)
	assert.NotEmpty(t, result.Tokens)
	assert.Less(t, time.Since(startTime), 10*time.Millisecond)

	//Second try. Should be < 10ms.
	startTime = time.Now()
	result, parsingErr, err = tryParseHyperScriptWithDenoService(ctx, code)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Nil(t, parsingErr) {
		return
	}

	assert.NotNil(t, result.NodeData)
	assert.NotEmpty(t, result.Tokens)
	assert.Less(t, time.Since(startTime), 5*time.Millisecond)
}
