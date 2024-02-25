package projectserver

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
	"github.com/inoxlang/inox/internal/sourcecontrol"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	GET_UNSTAGED_CHANGES_METHOD = "sourceControl/getUnstagedChanges"
	GET_STAGED_CHANGES_METHOD   = "sourceControl/getStagedChanges"

	STAGE_METHOD                = "sourceControl/stage"   //stage one or more files
	UNSTAGE_METHOD              = "sourceControl/unstage" //unstage one or more files
	COMMIT_IN_LOCAL_REPO_METHOD = "sourceControl/commit"
	GET_LAST_DEV_COMMIT_METHOD  = "sourceControl/getLastDevCommit"
	GET_DEV_LOG_METHOD          = "sourceControl/getDevLog"
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
	AbsolutePaths []string `json:"absolutePaths"`
}

type UnstageParams struct {
	AbsolutePaths []string `json:"absolutePaths"`
}

type CommitParams struct {
	Message string `json:"message"`
}

type GetLastDevCommitParams struct {
}

type GetLastDevCommitResponse struct {
	Commit CommitInfo `json:"commit,omitempty"`
}

type LogDevCommitsParams struct {
	FromHashHex string `json:"fromHashHex"`
}

type LogDevCommitsResponse struct {
	Commits []CommitInfo `json:"commits,omitempty"`
}

func registerSourceControlMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_UNSTAGED_CHANGES_METHOD,
		NewRequest: func() interface{} {
			return &GetUnstagedChangesParams{}
		},
		RateLimits: []int{2, 10, 50},
		Handler:    handleGetStagedOrUnstagedChanges,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_STAGED_CHANGES_METHOD,
		NewRequest: func() interface{} {
			return &GetStagedChangesParams{}
		},
		RateLimits: []int{2, 10, 50},
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

	server.OnCustom(jsonrpc.MethodInfo{
		Name: UNSTAGE_METHOD,
		NewRequest: func() interface{} {
			return &UnstageParams{}
		},
		RateLimits: []int{3, 100, 1000},
		Handler:    handleUnstageFileOrDir,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: COMMIT_IN_LOCAL_REPO_METHOD,
		NewRequest: func() interface{} {
			return &CommitParams{}
		},
		RateLimits: []int{1, 3, 10},
		Handler:    handleCommitInLocalRepo,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_LAST_DEV_COMMIT_METHOD,
		NewRequest: func() interface{} {
			return &GetLastDevCommitParams{}
		},
		RateLimits: []int{2, 10, 50},
		Handler:    handleGetLastDevCommit,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_DEV_LOG_METHOD,
		NewRequest: func() interface{} {
			return &LogDevCommitsParams{}
		},
		RateLimits: []int{2, 5, 30},
		Handler:    handleGetDevLog,
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

	for _, path := range params.AbsolutePaths {
		err := repo.Stage(path)
		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}

	}

	return nil, nil
}

func handleUnstageFileOrDir(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*UnstageParams)

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

	for _, path := range params.AbsolutePaths {
		err := repo.Unstage(path)

		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}

	}

	return nil, nil
}

func handleCommitInLocalRepo(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*CommitParams)

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

	err := repo.Commit(params.Message)

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	return nil, nil
}

func handleGetLastDevCommit(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	_ = req.(*GetLastDevCommitParams)

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

	commit, err := repo.LastDevCommit()

	if err != nil {

		if errors.Is(err, sourcecontrol.ErrNoReference) {
			return GetLastDevCommitResponse{}, nil
		}

		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	return GetLastDevCommitResponse{
		Commit: makeCommitInfo(commit),
	}, nil
}

func handleGetDevLog(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*LogDevCommitsParams)

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

	hash := plumbing.NewHash(params.FromHashHex)

	commits, err := repo.LogDevCommits(hash, nil, nil)

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	commitsInfo := utils.MapSlice(commits, makeCommitInfo)

	return LogDevCommitsResponse{
		Commits: commitsInfo,
	}, nil
}
