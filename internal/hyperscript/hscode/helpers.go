package hscode

type EventInfo struct {
	Name string
}

func LooksLikeNode(arg any) bool {
	goMap, ok := arg.(Map)
	if !ok {
		return false
	}
	_, ok = goMap["type"].(string)
	return ok
}

func IsNodeOfType(arg any, nodeType NodeType) bool {
	goMap, ok := arg.(Map)
	if !ok {
		return false
	}
	typeString, ok := goMap["type"].(string)
	return ok && typeString == string(nodeType)
}

func IsEmptyCommandListCommand(arg any) bool {
	return IsNodeOfType(arg, EmptyCommandListCommand)
}

func IsSymbolWithName(arg any, name string) bool {
	goMap, ok := arg.(Map)
	if !ok || !IsNodeOfType(arg, Symbol) {
		return false
	}
	s, ok := goMap["name"].(string)
	return ok && s == name
}

func GetSetCommandTarget(arg any) (any, bool) {
	if !IsNodeOfType(arg, SetCommand) {
		return nil, false
	}
	goMap := arg.(Map)
	target, ok := goMap["target"].(Map)
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
	goMap := target.(Map)
	return goMap["name"].(string), true
}

func GetProgramFeatures(node any) (features []any, success bool) {
	if !IsNodeOfType(node, HyperscriptProgram) {
		return
	}

	features, success = node.(Map)["features"].([]any)
	return
}

func GetTypeIfNode(arg any) NodeType {
	goMap, ok := arg.(Map)
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

	list, ok := node.(Map)["events"].([]any)
	if !ok {
		return
	}

	for _, event := range list {
		eventMap := event.(Map)

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
		goMap := node.(Map)
		start, ok := goMap["start"].(Map)
		if !ok {
			return
		}
		success = true
		commandList = flattenCommandList(start)
		return
	}
	return
}

func flattenCommandList(start Map) (list []any) {
	if IsEmptyCommandListCommand(start) {
		return
	}

	current := start

	for {
		list = append(list, current)

		next, ok := current["next"].(Map)
		if !ok || LooksLikeNode(next) {
			return
		}

		current = next
	}
}
