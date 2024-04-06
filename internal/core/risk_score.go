package core

import (
	"path/filepath"
	"reflect"
	"slices"
	"strconv"

	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/utils"
)

type RiskScore int

func (s RiskScore) ValueAndLevel() string {
	str := strconv.Itoa(int(s)) + " "
	switch {
	case s >= HIGH_RISK_SCORE_LEVEL:
		str += "(high)"
	case s >= MEDIUM_RISK_SCORE_LEVEL:
		str += "(medium)"
	default:
		str += "(low)"
	}

	return str
}

type BasePermissionRiskScore struct {
	Type  reflect.Type
	Kind  PermissionKind
	Score RiskScore
}

// The following risk score constants are intended to be a starting point, they may be adjusted based on additional research and feedback.
const (
	MAXIMUM_RISK_SCORE      = RiskScore(10_000)
	MEDIUM_RISK_SCORE_LEVEL = 300
	HIGH_RISK_SCORE_LEVEL   = 500
	UNKNOWN_PERM_RISK_SCORE = RiskScore(30)

	HOST_PATTERN_RISK_MULTIPLIER = RiskScore(4)
	HOST_RISK_MULTIPLIER         = RiskScore(3)
	URL_PATTERN_RISK_MULTIPLIER  = RiskScore(2)
	URL_RISK_MULTIPLIER          = RiskScore(1)

	UNKNOW_FILE_SENSITIVITY_MULTIPLIER         = 2
	UNKNOW_FILE_PATTERN_SENSITIVITY_MUTLIPLIER = 3

	HTTP_READ_PERM_RISK_SCORE    = 10
	HTTP_WRITE_PERM_RISK_SCORE   = 20
	HTTP_PROVIDE_PERM_RISK_SCORE = 20

	WS_READ_PERM_RISK_SCORE    = 10
	WS_WRITE_PERM_RISK_SCORE   = 20
	WS_PROVIDE_PERM_RISK_SCORE = 20

	FS_READ_PERM_RISK_SCORE  = 10
	FS_WRITE_PERM_RISK_SCORE = 20

	LTHREAD_PERM_RISK_SCORE = 2 //the creation of lthread is not risky, it's the number of goroutines that can be an issue

	CMD_PERM_RISK_SCORE = 30
)

var (
	HTTP_PERM_TYPE    = reflect.TypeOf(HttpPermission{})
	WS_PERM_TYPE      = reflect.TypeOf(WebsocketPermission{})
	FS_PERM_TYPE      = reflect.TypeOf(FilesystemPermission{})
	ROUTINE_PERM_TYPE = reflect.TypeOf(LThreadPermission{})
	CMD_PERM_TYPE     = reflect.TypeOf(CommandPermission{})

	DEFAULT_PERM_RISK_SCORES = map[reflect.Type][]BasePermissionRiskScore{
		HTTP_PERM_TYPE: {
			{HTTP_PERM_TYPE, permbase.Read, HTTP_READ_PERM_RISK_SCORE},
			{HTTP_PERM_TYPE, permbase.Write, HTTP_WRITE_PERM_RISK_SCORE},
			{HTTP_PERM_TYPE, permbase.Provide, HTTP_WRITE_PERM_RISK_SCORE},
		},

		WS_PERM_TYPE: {
			{WS_PERM_TYPE, permbase.Read, WS_READ_PERM_RISK_SCORE},
			{WS_PERM_TYPE, permbase.Write, WS_WRITE_PERM_RISK_SCORE},
			{WS_PERM_TYPE, permbase.Provide, WS_WRITE_PERM_RISK_SCORE},
		},

		FS_PERM_TYPE: {
			{FS_PERM_TYPE, permbase.Read, FS_READ_PERM_RISK_SCORE},
			{FS_PERM_TYPE, permbase.Write, FS_WRITE_PERM_RISK_SCORE},
		},

		ROUTINE_PERM_TYPE: {
			{ROUTINE_PERM_TYPE, permbase.Create, LTHREAD_PERM_RISK_SCORE},
		},

		CMD_PERM_TYPE: {
			{CMD_PERM_TYPE, permbase.Use, CMD_PERM_RISK_SCORE},
		},
	}

	//TODO: move constants to an embedded file.
	//TODO: handle case where the a virtual system is used.
	FILE_SENSITIVITY_MULTIPLIERS = []struct {
		PathPattern
		Multiplier int
	}{
		{"/home/*/.*", 3},      //dot files in home directories
		{"/home/*/.*/**/*", 3}, //dot directories in home directories
		{"/etc/**/*", 3},
		{"/usr/**/*", 4},
		{"/bin/**/*", 4},
		{"/sbin/**/*", 4},
		{"/*", 4},
	}
)

// ComputeProgramRiskScore computes the risk score for a prepared program. First the risk score for each permission
// is computed, then scores of permissions of the same type are summed and finally the remaining scores are multiplied together.
// The current logic is intended to be a starting point, it may be adjusted based on additional research and feedback.
func ComputeProgramRiskScore(mod *Module, manifest *Manifest) (totalScore RiskScore, requiredPerms []Permission) {
	permTypeRiskScores := map[reflect.Type]RiskScore{}
	requiredPerms = slices.Clone(manifest.RequiredPermissions)

	for _, preinitFilePerm := range manifest.PreinitFiles {
		requiredPerms = append(requiredPerms, preinitFilePerm.RequiredPermission)
	}

	for _, requiredPerm := range requiredPerms {
		if _, ok := requiredPerm.(GlobalVarPermission); ok { //ignore
			continue
		}
		permRisk := ComputePermissionRiskScore(requiredPerm)
		permTypeRiskScores[reflect.TypeOf(requiredPerm)] += permRisk
	}

	totalScore = 1
	combinedHttpWsScore := RiskScore(1)

	for permType, score := range permTypeRiskScores {
		if totalScore > MAXIMUM_RISK_SCORE/score {
			return MAXIMUM_RISK_SCORE, requiredPerms
		}

		//Special case: the HTTP and Websocket scores are added together because they are almost equivalent.
		//The combined score is multiplied with the total score after the loop.
		switch permType {
		case HTTP_PERM_TYPE, WS_PERM_TYPE:
			if combinedHttpWsScore <= 1 {
				combinedHttpWsScore = score
			} else {
				combinedHttpWsScore = max(score, combinedHttpWsScore) + min(score, combinedHttpWsScore)
			}
			continue
		}

		totalScore *= score
	}

	if totalScore > MAXIMUM_RISK_SCORE/combinedHttpWsScore {
		return MAXIMUM_RISK_SCORE, requiredPerms
	}

	totalScore *= combinedHttpWsScore

	return totalScore, requiredPerms
}

func ComputePermissionRiskScore(perm Permission) RiskScore {
	majorPermKind := perm.Kind().Major()
	permType := reflect.TypeOf(perm)

	permRiskScores, ok := DEFAULT_PERM_RISK_SCORES[permType]
	if !ok {
		return UNKNOWN_PERM_RISK_SCORE
	}

	var score RiskScore = UNKNOWN_PERM_RISK_SCORE

	for _, permRiskScore := range permRiskScores {
		if permRiskScore.Type == permType && permRiskScore.Kind.Major() == majorPermKind {
			score = permRiskScore.Score
		}
	}

	switch p := perm.(type) {
	case GlobalVarPermission:
		return 1
	case HttpPermission:
		if p.AnyEntity {
			score *= HOST_PATTERN_RISK_MULTIPLIER
			break
		}

		switch p.Entity.(type) {
		case HostPattern:
			score *= HOST_PATTERN_RISK_MULTIPLIER
			//TODO: if subdomains: is the domain trustable ?
		case Host:
			score *= HOST_RISK_MULTIPLIER
			//TODO: is the domain trustable ?
		case URLPattern:
			score *= URL_PATTERN_RISK_MULTIPLIER
			//TODO: is the domain trustable ?
		case URL:
			score *= URL_RISK_MULTIPLIER
			//TODO: is the domain trustable ?
		default:
			panic(ErrUnreachable)
		}
	case WebsocketPermission:
		switch p.Endpoint.(type) {
		case Host:
			score *= HOST_RISK_MULTIPLIER
			//TODO: is the domain trustable ?
		case URL:
			score *= URL_RISK_MULTIPLIER
			//TODO: is the domain trustable ?
		default:
			panic(ErrUnreachable)
		}
	case FilesystemPermission:
		switch entity := p.Entity.(type) {
		case PathPattern:
			score *= RiskScore(GetPathPatternSensitivityMultiplier(entity))
		case Path:
			score *= RiskScore(GetPathSensitivityMultiplier(entity))
		default:
			panic(ErrUnreachable)
		}
	}

	return score
}

func GetPathSensitivityMultiplier(pth Path) int {
	for _, sensitivity := range FILE_SENSITIVITY_MULTIPLIERS {
		if utils.Must(filepath.Match(string(sensitivity.PathPattern), string(pth))) {
			return sensitivity.Multiplier
		}
	}

	return UNKNOW_FILE_SENSITIVITY_MULTIPLIER
}

func GetPathPatternSensitivityMultiplier(patt PathPattern) int {
	var maxMultiplier int = UNKNOW_FILE_PATTERN_SENSITIVITY_MUTLIPLIER
	for _, sensitivity := range FILE_SENSITIVITY_MULTIPLIERS {
		if sensitivity.PathPattern == patt {
			maxMultiplier = max(maxMultiplier, sensitivity.Multiplier)
		}
	}

	//TODO: support comparing globbing patterns
	return maxMultiplier
}
