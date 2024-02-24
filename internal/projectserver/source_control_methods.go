package projectserver

import (
	"context"
	"errors"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
	"github.com/inoxlang/inox/internal/sourcecontrol"
)

const (
	GET_UNSTAGED_CHANGES_METHOD = "sourceControl/getUnstagedChanges"
	GET_STAGED_CHANGES_METHOD   = "sourceControl/getStagedChanges"
	STAGE_METHOD                = "sourceControl/stage" //stage file or directory
	COMMIT_IN_LOCAL_REPO_METHOD = "sourceControl/commit"
	PUSH_TO_REPO_METHOD         = "sourceControl/push"
	PULL_FROM_REPO_METHOD       = "sourceControl/pull"
)

var (
	ErrNoSrcControlRepository = errors.New("no source control repository")
)

type GetUnstagedChangesParams struct{}

type GetUnstagedChangesResponse struct {
	Changes []SourceControlFileChange `json:"changes"`
}

type GetStagedChangesParams struct{}

type GetStagedChangesResponse struct {
	Changes []SourceControlFileChange `json:"changes"`
}

type SourceControlFileChange struct {
	AbsoluteFilepath string `json:"absoluteFilepath"`
	Status           string `json:"status"`
}

type StageParams struct {
	AbsolutePath string `json:"absolutePath"`
}

func registerSourceControlMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_UNSTAGED_CHANGES_METHOD,
		NewRequest: func() interface{} {
			return &GetUnstagedChangesParams{}
		},
		RateLimits: []int{2, 5, 50},
		Handler:    handleGetStagedOrUnstagedChanges,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_STAGED_CHANGES_METHOD,
		NewRequest: func() interface{} {
			return &GetStagedChangesParams{}
		},
		RateLimits: []int{2, 5, 50},
		Handler:    handleGetStagedOrUnstagedChanges,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: STAGE_METHOD,
		NewRequest: func() interface{} {
			return &StageParams{}
		},
		RateLimits: []int{3, 100, 1000},
		Handler:    handleStageFileOrDir,
	})
}

func handleGetStagedOrUnstagedChanges(ctx context.Context, req interface{}) (interface{}, error) {
	_, ok := req.(*GetUnstagedChangesParams)
	if ok { //unstaged
		changes, err := handleGetChanges(ctx, false, req)
		if err != nil {
			return nil, err
		}

		return GetUnstagedChangesResponse{
			Changes: changes,
		}, nil
	}

	//staged

	changes, err := handleGetChanges(ctx, true, req)
	if err != nil {
		return nil, err
	}

	return GetStagedChangesResponse{
		Changes: changes,
	}, nil
}

func handleGetChanges(ctx context.Context, staged bool, req interface{}) ([]SourceControlFileChange, error) {
	session := jsonrpc.GetSession(ctx)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	fls := sessionData.filesystem
	repo := sessionData.repository
	sessionData.lock.Unlock()
	//----------------------------------------

	if fls == nil {
		return nil, errors.New(string(FsNoFilesystem))
	}

	if repo == nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: ErrNoSrcControlRepository.Error(),
		}
	}

	var changes []sourcecontrol.Change
	var err error
	if staged {
		changes, err = repo.GetStagedChanges()
		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}
	} else {
		changes, err = repo.GetUnstagedChanges()
		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}
	}

	var respChanges []SourceControlFileChange

	for _, change := range changes {
		respChanges = append(respChanges, SourceControlFileChange{
			AbsoluteFilepath: change.AbsolutePath,
			Status:           string(rune(change.Code)),
		})
	}

	return respChanges, nil
}

func handleStageFileOrDir(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*StageParams)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	fls := sessionData.filesystem
	repo := sessionData.repository
	sessionData.lock.Unlock()
	//----------------------------------------

	if fls == nil {
		return nil, errors.New(string(FsNoFilesystem))
	}

	if repo == nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: ErrNoSrcControlRepository.Error(),
		}
	}

	err := repo.Stage(params.AbsolutePath)

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	return nil, nil
}
