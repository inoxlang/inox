package projectserver

import (
	"context"

	"github.com/inoxlang/inox/internal/learn"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
)

const (
	GET_TUTORIAL_SERIES_METHOD = "learn/getTutorialSeries"
	GET_LEARN_INFO_METHOD      = "learn/getInfo"
)

type GetTutorialSeriesParamss struct {
}

type TutorialSeriesList struct {
	TutorialSeries []learn.TutorialSeries `json:"tutorialSeries"`
}

type GetLearnInfoParams struct {
}

type LearnInfo struct {
}

func registerLearningMethodHandlers(server *lsp.Server) {
	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_TUTORIAL_SERIES_METHOD,
		NewRequest: func() interface{} {
			return &GetTutorialSeriesParamss{}
		},
		AvoidLogging: true,
		Handler: func(callCtx context.Context, req interface{}) (interface{}, error) {
			return TutorialSeriesList{
				TutorialSeries: learn.TUTORIAL_SERIES,
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: GET_LEARN_INFO_METHOD,
		NewRequest: func() interface{} {
			return &GetLearnInfoParams{}
		},
		Handler: func(callCtx context.Context, req interface{}) (interface{}, error) {
			return LearnInfo{}, nil
		},
	})

}
