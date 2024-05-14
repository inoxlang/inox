package hscode

import (
	"errors"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

func GetNodeSpan(n JSONMap) (int32, int32) {
	if !LooksLikeNode(n) {
		panic(errors.New("expected argument to be a node"))
	}
	startToken := n["startToken"].(JSONMap)
	endToken := n["endToken"].(JSONMap)

	startTokenStart, ok := startToken["start"].(float64)
	if !ok {
		startTokenStart = 0
	}

	endTokenEnd, ok := endToken["end"].(float64)
	if !ok {
		endTokenEnd = 0
	}

	start := int32(startTokenStart)
	end := int32(endTokenEnd)
	return start, end
}

type EventInfo struct {
	Name string
}

func LooksLikeNode(arg any) bool {
	goMap, ok := arg.(JSONMap)
	if !ok {
		return false
	}
	_, hasType := goMap["type"].(string)
	if !hasType {
		return false
	}
	_, hasStringValue := goMap["value"].(string)
	if hasStringValue {
		_, ok := goMap["start"].(float64)
		if ok {
			return false //token
		}
		_, ok = goMap["line"].(float64)
		if ok {
			return false //token
		}
	}

	return true
}

func IsNodeOfType(arg any, nodeType NodeType) bool {
	goMap, ok := arg.(JSONMap)
	if !ok {
		return false
	}
	typeString, ok := goMap["type"].(string)
	return ok && typeString == string(nodeType)
}

func AssertIsNodeOfType(arg any, nodeType NodeType) {
	if !IsNodeOfType(arg, nodeType) {
		panic(fmt.Errorf("expected argument to be a node of type %s", nodeType))
	}
}

func IsEmptyCommandListCommand(arg any) bool {
	return IsNodeOfType(arg, EmptyCommandListCommand)
}

func IsSymbolWithName(arg any, name string) bool {
	if !IsNodeOfType(arg, Symbol) {
		return false
	}
	s, ok := arg.(JSONMap)["name"].(string)
	return ok && s == name
}

func GetSetCommandTarget(arg any) (any, bool) {
	if !IsNodeOfType(arg, SetCommand) {
		return nil, false
	}
	target, ok := arg.(JSONMap)["target"].(JSONMap)
	if !ok {
		return nil, false
	}
	return target, true
}

func GetSetCommandTargetName(arg any) (string, bool) {
	target, ok := GetSetCommandTarget(arg)
	if !ok || !IsNodeOfType(target, Symbol) {
		return "", false
	}
	return target.(JSONMap)["name"].(string), true
}

func GetSymbolName(arg any) string {
	AssertIsNodeOfType(arg, Symbol)
	return arg.(JSONMap)["name"].(string)
}

func GetAttributeRefName(arg any) string {
	AssertIsNodeOfType(arg, AttributeRef)
	return arg.(JSONMap)["name"].(string)
}

func IsTarget(target, arg JSONMap) bool {
	if !LooksLikeNode(arg) {
		return false
	}
	actualTarget, ok := arg["target"].(JSONMap)
	return ok && utils.SamePointer(target, actualTarget)
}

func GetProgramFeatures(node any) (features []any, success bool) {
	if !IsNodeOfType(node, HyperscriptProgram) {
		return
	}

	features, success = node.(JSONMap)["features"].([]any)
	return
}

func GetTypeIfNode(arg any) NodeType {
	goMap, ok := arg.(JSONMap)
	if !ok {
		return ""
	}
	nodeType, _ := goMap["type"].(string)
	return NodeType(nodeType)
}

func GetOnFeatureEvents(node any) (events []EventInfo, success bool) {
	if !IsNodeOfType(node, OnFeature) {
		return
	}

	list, ok := node.(JSONMap)["events"].([]any)
	if !ok {
		return
	}

	for _, event := range list {
		eventMap := event.(JSONMap)

		eventInfo := EventInfo{
			Name: eventMap["on"].(string),
		}
		events = append(events, eventInfo)
	}

	return
}

func GetCommandList(node any) (commandList []any, success bool) {
	nodeType := GetTypeIfNode(node)
	switch nodeType {
	case InitFeature, OnFeature, DefFeature:
		goMap := node.(JSONMap)
		start, ok := goMap["start"].(JSONMap)
		if !ok {
			return
		}
		success = true
		commandList = flattenCommandList(start)
		return
	case EmptyCommandListCommand:
		return nil, true
	}
	if strings.HasSuffix(string(nodeType), "Command") {
		success = true
		commandList = []any{node}
		return
	}
	return
}

func flattenCommandList(start JSONMap) (list []any) {
	if IsEmptyCommandListCommand(start) {
		return
	}

	current := start

	for {
		list = append(list, current)

		next, ok := current["next"].(JSONMap)
		if !ok || LooksLikeNode(next) {
			return
		}

		current = next
	}
}
