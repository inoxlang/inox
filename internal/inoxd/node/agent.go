package node

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"github.com/rs/zerolog"
)

const (
	AGENT_LOG_SRC = "node-agent"
)

// A node Agent is responsible for managing Inox applications and services running on a single node.
type Agent struct {
	lock sync.Mutex

	goCtx        context.Context
	logger       zerolog.Logger
	config       AgentConfig
	applications map[ApplicationName]*Application
}

type AgentParameters struct {
	GoCtx  context.Context
	Logger zerolog.Logger
	Config AgentConfig
}

type AgentConfig struct {
	OsProdDir core.Path
}

func NewAgent(args AgentParameters) (*Agent, error) {
	//check configuration

	prodDir := args.Config.OsProdDir
	if !prodDir.IsDirPath() {
		return nil, fmt.Errorf("filepath %q provided as prod directory", prodDir.UnderlyingString())
	}

	_, err := os.ReadDir(prodDir.UnderlyingString())
	if err != nil {
		return nil, fmt.Errorf("failed to read entries of the prod directory (%s): %w", prodDir, err)
	}

	//TODO: add lock file

	agent := &Agent{
		config:       args.Config,
		goCtx:        args.GoCtx,
		logger:       args.Logger,
		applications: map[ApplicationName]*Application{},
	}

	return agent, nil
}
