package internal

import (
	"path/filepath"
	"reflect"
	"strconv"

	permkind "github.com/inoxlang/inox/internal/permkind"
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

	FS_READ_PERM_RISK_SCORE  = 10
	FS_WRITE_PERM_RISK_SCORE = 20

	ROUTINE_PERM_RISK_SCORE = 2 //the creation of coroutines is not risky, it's the number of goroutines that can be an issue

	CMD_PERM_RISK_SCORE = 30
)

var (
	HTTP_PERM_TYPE    = reflect.TypeOf(HttpPermission{})
	FS_PERM_TYPE      = reflect.TypeOf(FilesystemPermission{})
	ROUTINE_PERM_TYPE = reflect.TypeOf(RoutinePermission{})
	CMD_PERM_TYPE     = reflect.TypeOf(CommandPermission{})

	DEFAULT_PERM_RISK_SCORES = map[reflect.Type][]BasePermissionRiskScore{
		HTTP_PERM_TYPE: {
			{HTTP_PERM_TYPE, permkind.Read, HTTP_READ_PERM_RISK_SCORE},
			{HTTP_PERM_TYPE, permkind.Write, HTTP_WRITE_PERM_RISK_SCORE},
			{HTTP_PERM_TYPE, permkind.Provide, HTTP_WRITE_PERM_RISK_SCORE},
		},
		FS_PERM_TYPE: {
			{FS_PERM_TYPE, permkind.Read, FS_READ_PERM_RISK_SCORE},
			{FS_PERM_TYPE, permkind.Write, FS_WRITE_PERM_RISK_SCORE},
		},
		ROUTINE_PERM_TYPE: {
			{ROUTINE_PERM_TYPE, permkind.Create, ROUTINE_PERM_RISK_SCORE},
		},
		CMD_PERM_TYPE: {
			{CMD_PERM_TYPE, permkind.Use, CMD_PERM_RISK_SCORE},
		},
	}

	//TODO: move constants to an embedded file
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
// is computed, then scores of permissions of the same type are summed and finally the remaining score are multiplied.
func ComputeProgramRiskScore(mod *Module, manifest *Manifest) (totalScore RiskScore) {
	permTypeRiskScores := map[reflect.Type]RiskScore{}

	for _, requiredPerm := range manifest.RequiredPermissions {
		if _, ok := requiredPerm.(GlobalVarPermission); ok { //ignore
			continue
		}
		permRisk := ComputePermissionRiskScore(requiredPerm)
		permTypeRiskScores[reflect.TypeOf(requiredPerm)] += permRisk
	}

	totalScore = 1
	for _, score := range permTypeRiskScores {
		if totalScore > MAXIMUM_RISK_SCORE/score {
			return MAXIMUM_RISK_SCORE
		}
		totalScore *= score
	}

	return totalScore
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
			maxMultiplier = utils.Max(maxMultiplier, sensitivity.Multiplier)
		}
	}

	//TODO: support comparing globbing patterns
	return maxMultiplier
}
