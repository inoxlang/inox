package staticcheck

import (
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/inoxconsts"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	MODULE_KIND_TO_ALLOWED_SECTION_NAMES = map[inoxmod.Kind][]string{
		inoxmod.UnspecifiedModuleKind: inoxconsts.MANIFEST_SECTION_NAMES,
		inoxmod.ApplicationModule:     inoxconsts.MANIFEST_SECTION_NAMES,
		inoxmod.SpecModule:            {inoxconsts.MANIFEST_KIND_SECTION_NAME, inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
		inoxmod.TestSuiteModule:       {inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
		inoxmod.TestCaseModule:        {inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
	}
)

type ManifestStaticCheckArguments struct {
	ObjectLit             *ast.ObjectLiteral
	IgnoreUnknownSections bool
	ModuleKind            inoxmod.Kind
	OnError               func(n ast.Node, msg string)
}

func CheckManifestObject(args ManifestStaticCheckArguments) {
	objLit := args.ObjectLit
	ignoreUnknownSections := args.IgnoreUnknownSections
	onError := args.OnError

	ast.Walk(objLit, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		switch n := node.(type) {
		case *ast.ObjectLiteral:
			if len(n.SpreadElements) != 0 {
				onError(n, text.NO_SPREAD_IN_MANIFEST)
			}
			shallowCheckObjectRecordProperties(n.Properties, nil, true, func(n ast.Node, msg string) {
				onError(n, msg)
			})
		case *ast.RecordLiteral:
			if len(n.SpreadElements) != 0 {
				onError(n, text.NO_SPREAD_IN_MANIFEST)
			}
			shallowCheckObjectRecordProperties(n.Properties, nil, false, func(n ast.Node, msg string) {
				onError(n, msg)
			})
		case *ast.ListLiteral:
			if n.HasSpreadElements() {
				onError(n, text.NO_SPREAD_IN_MANIFEST)
			}
		}

		return ast.ContinueTraversal, nil
	}, nil)

	for _, p := range objLit.Properties {
		if p.HasNoKey() {
			onError(p, text.ELEMENTS_NOT_ALLOWED_IN_MANIFEST)
			continue
		}

		sectionName := p.Name()
		allowedSectionNames := MODULE_KIND_TO_ALLOWED_SECTION_NAMES[args.ModuleKind]
		if !slices.Contains(allowedSectionNames, sectionName) {
			onError(p.Key, text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind(sectionName, args.ModuleKind))
			continue
		}

		switch sectionName {
		case inoxconsts.MANIFEST_KIND_SECTION_NAME:
			kindName, ok := inoxmod.GetUncheckedModuleKindNameFromNode(p.Value)
			if !ok {
				onError(p.Key, text.KIND_SECTION_SHOULD_BE_A_STRING_LITERAL)
				continue
			}

			kindInManifest, err := inoxmod.ParseModuleKind(kindName)
			if err != nil {
				onError(p.Key, inoxmod.ErrInvalidModuleKind.Error())
				continue
			}

			if kindInManifest.IsEmbedded() {
				onError(p.Key, text.INVALID_KIND_SECTION_EMBEDDED_MOD_KINDS_NOT_ALLOWED)
				continue
			}

			if kindInManifest == inoxmod.UnspecifiedModuleKind {
				onError(p.Key, text.THE_UNSPECIFIED_MOD_KIND_NAME_CANNOT_BE_USED_IN_THE_MANIFEST)
				continue
			}

			switch args.ModuleKind {
			case inoxmod.SpecModule:
				if kindInManifest != args.ModuleKind {
					onError(p.Value, text.MOD_KIND_SPECIFIED_IN_MANIFEST_SHOULD_BE_SPEC_OR_SHOULD_BE_OMITTED)
				}
			case inoxmod.UnspecifiedModuleKind:
				//ok
			default:
				if kindInManifest != args.ModuleKind {
					onError(p.Value, text.MOD_KIND_NOT_EQUAL_TO_KIND_DETERMINED_DURING_PARSING)
				}
			}
		case inoxconsts.MANIFEST_PERMS_SECTION_NAME:
			if obj, ok := p.Value.(*ast.ObjectLiteral); ok {
				CheckPermissionListingObject(obj, onError)
			} else {
				onError(p, text.PERMS_SECTION_SHOULD_BE_AN_OBJECT)
			}
		case inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME:
			dict, ok := p.Value.(*ast.DictionaryLiteral)
			if !ok {
				onError(p, text.HOST_DEFS_SECTION_SHOULD_BE_A_DICT)
				continue
			}

			hasErrors := false

			ast.Walk(dict, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
				if node == dict {
					return ast.ContinueTraversal, nil
				}

				switch n := node.(type) {
				case *ast.ObjectLiteral, *ast.ObjectProperty:
				case *ast.DictionaryEntry, ast.SimpleValueLiteral,
					*ast.IdentifierMemberExpression:
				default:
					hasErrors = true
					onError(n, text.FmtForbiddenNodeInHostDefinitionsSection(n))
				}

				return ast.ContinueTraversal, nil
			}, nil)

			if !hasErrors {
				staticallyCheckHostDefinitionFnRegistryLock.Lock()
				defer staticallyCheckHostDefinitionFnRegistryLock.Unlock()

				for _, entry := range dict.Entries {
					key := entry.Key

					switch k := key.(type) {
					case *ast.InvalidURL:
					case *ast.HostLiteral:
						host := utils.Must(EvalSimpleValueLiteral(k)).(Host)
						fn, ok := staticallyCheckHostDefinitionDataFnRegistry[GetHostScheme(host)]
						if ok {
							errMsg := fn(entry.Value)
							if errMsg != "" {
								onError(entry.Value, errMsg)
							}
						} else {
							onError(k, text.HOST_SCHEME_NOT_SUPPORTED)
						}
					default:
						onError(k, text.HOST_DEFS_SECTION_SHOULD_BE_A_DICT)
					}
				}
			}
		case inoxconsts.MANIFEST_LIMITS_SECTION_NAME:
			obj, ok := p.Value.(*ast.ObjectLiteral)

			if !ok {
				onError(p, text.LIMITS_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			ast.Walk(obj, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
				if node == obj {
					return ast.ContinueTraversal, nil
				}

				switch n := node.(type) {
				case *ast.ObjectProperty, ast.SimpleValueLiteral:
				default:
					onError(n, text.FmtForbiddenNodeInLimitsSection(n))
				}

				return ast.ContinueTraversal, nil
			}, nil)
		case inoxconsts.MANIFEST_ENV_SECTION_NAME:

			if args.ModuleKind.IsEmbedded() {
				onError(p, text.ENV_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			patt, ok := p.Value.(*ast.ObjectPatternLiteral)

			if !ok {
				onError(p, text.ENV_SECTION_SHOULD_BE_AN_OBJECT_PATTERN)
				continue
			}

			ast.Walk(patt, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
				if node == patt {
					return ast.ContinueTraversal, nil
				}

				switch n := node.(type) {
				case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression,
					*ast.ObjectPatternProperty, *ast.PatternCallExpression, ast.SimpleValueLiteral:
				default:
					onError(n, text.FmtForbiddenNodeInEnvSection(n))
				}

				return ast.ContinueTraversal, nil
			}, nil)
		case inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME:
			if args.ModuleKind.IsEmbedded() {
				onError(p, text.PREINIT_FILES_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			obj, ok := p.Value.(*ast.ObjectLiteral)

			if !ok {
				onError(p, text.PREINIT_FILES_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			CheckPreinitFilesObject(obj, onError)
		case inoxconsts.MANIFEST_PARAMS_SECTION_NAME:
			if args.ModuleKind.IsEmbedded() {
				onError(p, text.PARAMS_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			obj, ok := p.Value.(*ast.ObjectLiteral)

			if !ok {
				onError(p, text.PARAMS_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			checkParametersObject(obj, onError)
		default:
			if !ignoreUnknownSections {
				onError(p, text.FmtUnknownSectionOfManifest(p.Name()))
			}
		}
	}

}

func CheckPermissionListingObject(objLit *ast.ObjectLiteral, onError func(n ast.Node, msg string)) {
	ast.Walk(objLit, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		switch n := node.(type) {
		case *ast.ObjectLiteral, *ast.ListLiteral, *ast.DictionaryLiteral, *ast.DictionaryEntry, *ast.ObjectProperty,
			ast.SimpleValueLiteral, *ast.PatternIdentifierLiteral, *ast.URLExpression, *ast.PathPatternExpression,
			*ast.Variable:
		default:
			onError(n, text.FmtForbiddenNodeInPermListing(n))
		}

		return ast.ContinueTraversal, nil
	}, nil)

	for _, p := range objLit.Properties {
		if p.HasNoKey() {
			onError(p, text.ELEMENTS_NOT_ALLOWED_IN_PERMS_SECTION)
			continue
		}

		propName := p.Name()
		permKind, ok := permbase.PermissionKindFromString(propName)
		if !ok {
			onError(p.Key, text.FmtNotValidPermissionKindName(p.Name()))
			continue
		}
		checkSingleKindPermissions(permKind, p.Value, onError)
	}
}

func checkSingleKindPermissions(permKind permbase.PermissionKind, desc ast.Node, onError func(n ast.Node, msg string)) {
	checkSingleItem := func(node ast.Node) {
		switch n := node.(type) {
		case *ast.AbsolutePathExpression:
		case *ast.AbsolutePathLiteral:
		case *ast.RelativePathLiteral:
			onError(n, text.FmtOnlyAbsPathsAreAcceptedInPerms(n.Raw))
		case *ast.AbsolutePathPatternLiteral:
		case *ast.RelativePathPatternLiteral:
			onError(n, text.FmtOnlyAbsPathPatternsAreAcceptedInPerms(n.Raw))
		case *ast.URLExpression:
		case *ast.URLLiteral:
		case *ast.URLPatternLiteral:
		case *ast.HostLiteral:
		case *ast.HostPatternLiteral:
		case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceIdentifierLiteral:
		case *ast.Variable, *ast.IdentifierLiteral:

		case *ast.DoubleQuotedStringLiteral, *ast.MultilineStringLiteral, *ast.UnquotedStringLiteral:
			s := n.(ast.SimpleValueLiteral).ValueString()

			if len(s) <= 1 {
				onError(n, text.NO_PERM_DESCRIBED_BY_STRINGS)
				break
			}

			msg := text.NO_PERM_DESCRIBED_BY_STRINGS + ", "
			startsWithPercent := s[0] == '%'
			stringNoPercent := s
			if startsWithPercent {
				stringNoPercent = s[1:]
			}

			for _, prefix := range []string{"/", "./", "../"} {
				if strings.HasPrefix(stringNoPercent, prefix) {
					if startsWithPercent {
						msg += text.MAYBE_YOU_MEANT_TO_WRITE_A_PATH_PATTERN_LITERAL
					} else {
						msg += text.MAYBE_YOU_MEANT_TO_WRITE_A_PATH_LITERAL
					}
					break
				}
			}

			for _, prefix := range []string{"https://", "http://"} {
				if strings.HasPrefix(stringNoPercent, prefix) {
					if startsWithPercent {
						msg += text.MAYBE_YOU_MEANT_TO_WRITE_A_URL_PATTERN_LITERAL
					} else {
						msg += text.MAYBE_YOU_MEANT_TO_WRITE_A_URL_LITERAL
					}
					break
				}
			}

			onError(n, msg)
		default:
			onError(n, text.NO_PERM_DESCRIBED_BY_THIS_TYPE_OF_VALUE)
		}
	}

	switch v := desc.(type) {
	case *ast.ListLiteral:
		for _, elem := range v.Elements {
			checkSingleItem(elem)
		}
	case *ast.ObjectLiteral:
		for _, prop := range v.Properties {
			if prop.HasNoKey() {
				checkSingleItem(prop.Value)
			} else {
				typeName := prop.Name()

				//TODO: finish
				switch typeName {
				case "dns":
				case "tcp":
				case "globals":
				case "env":
				case "threads":
				case "system-graph":
				case "commands":
				case "values":
				case "custom":
				default:
					onError(prop.Value, text.FmtCannotInferPermission(permKind.String(), typeName))
				}
			}
		}
	default:
		checkSingleItem(v)
	}

}

func CheckPreinitFilesObject(obj *ast.ObjectLiteral, onError func(n ast.Node, msg string)) {

	hasForbiddenNodes := false

	ast.Walk(obj, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		if node == obj {
			return ast.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression, *ast.ObjectLiteral,
			*ast.ObjectProperty, *ast.PatternCallExpression, ast.SimpleValueLiteral,
			*ast.AbsolutePathExpression, *ast.RelativePathExpression:
		default:
			onError(n, text.FmtForbiddenNodeInPreinitFilesSection(n))
			hasForbiddenNodes = true
		}

		return ast.ContinueTraversal, nil
	}, nil)

	if hasForbiddenNodes {
		return
	}

	for _, p := range obj.Properties {
		if p.Value == nil {
			continue
		}
		fileDesc, ok := p.Value.(*ast.ObjectLiteral)
		if !ok {
			onError(p.Value, text.PREINIT_FILES__FILE_CONFIG_SHOULD_BE_AN_OBJECT)
			continue
		}

		pathNode, ok := fileDesc.PropValue(inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME)

		if !ok {
			onError(p, text.FmtMissingPropInPreinitFileDescription(inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME, p.Name()))
		} else {
			switch pathNode.(type) {
			case *ast.AbsolutePathLiteral, *ast.AbsolutePathExpression:
			default:
				onError(p, text.PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH)
			}
		}

		if !fileDesc.HasNamedProp(inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME) {
			onError(p, text.FmtMissingPropInPreinitFileDescription(inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME, p.Name()))
		}

	}
}

func checkParametersObject(objLit *ast.ObjectLiteral, onError func(n ast.Node, msg string)) {

	ast.Walk(objLit, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		if node == objLit {
			return ast.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case
			*ast.ObjectProperty, *ast.ObjectLiteral, *ast.ListLiteral,
			*ast.OptionExpression,
			ast.SimpleValueLiteral,
			//patterns
			*ast.PatternCallExpression,
			*ast.ListPatternLiteral, *ast.TuplePatternLiteral,
			*ast.ObjectPatternLiteral, *ast.ObjectPatternProperty, *ast.RecordPatternLiteral,
			*ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression, *ast.PatternNamespaceIdentifierLiteral,
			*ast.PatternConversionExpression,
			*ast.PatternUnion,
			*ast.PathPatternExpression, *ast.AbsolutePathPatternLiteral, *ast.RelativePathPatternLiteral,
			*ast.URLPatternLiteral, *ast.HostPatternLiteral, *ast.OptionalPatternExpression,
			*ast.OptionPatternLiteral, *ast.FunctionPatternExpression, *ast.NamedSegmentPathPatternLiteral:
		default:
			onError(n, text.FmtForbiddenNodeInParametersSection(n))
		}

		return ast.ContinueTraversal, nil
	}, nil)

	positionalParamsEnd := false

	for _, prop := range objLit.Properties {
		if !prop.HasNoKey() { // non positional parameter
			positionalParamsEnd = true

			propValue := prop.Value
			optionPattern, isOptionPattern := prop.Value.(*ast.OptionPatternLiteral)
			if isOptionPattern {
				propValue = optionPattern.Value
			}

			switch propVal := propValue.(type) {
			case *ast.ObjectLiteral:
				if isOptionPattern {
					break
				}

				missingPropertyNames := []string{"pattern"}

				for _, paramDescProp := range propVal.Properties {
					if paramDescProp.HasNoKey() {
						continue
					}
					name := paramDescProp.Name()

					for i, name := range missingPropertyNames {
						if name == paramDescProp.Name() {
							missingPropertyNames[i] = ""
						}
					}

					switch name {
					case inoxconsts.MANIFEST_PARAM__PATTERN_PROPNAME:
						if !ast.NodeIsPattern(paramDescProp.Value) {
							onError(paramDescProp, "the .pattern of a non positional parameter should be a named pattern or a pattern literal")
						}
					case "default":
					case "char-name":
						switch paramDescProp.Value.(type) {
						case *ast.RuneLiteral:
						default:
							onError(paramDescProp, "the .char-name of a non positional parameter should be a rune literal")
						}
					case "description":
						switch paramDescProp.Value.(type) {
						case *ast.DoubleQuotedStringLiteral, *ast.MultilineStringLiteral:
						default:
							onError(paramDescProp, "the .description of a non positional parameter should be a string literal")
						}
					}
				}

				missingPropertyNames = utils.FilterSlice(missingPropertyNames, func(s string) bool { return s != "" })
				if len(missingPropertyNames) > 0 {
					onError(prop, "missing properties in description of non positional parameter: "+strings.Join(missingPropertyNames, ", "))
				}
			default:
				if !ast.NodeIsPattern(prop.Value) {
					onError(prop, "the description of a non positional parameter should be a named pattern or a pattern literal")
				}
			}

		} else if positionalParamsEnd {
			onError(prop, "elements (values with no key) describe positional parameters, all implict key properties should be at the top of the 'parameters' section")
		} else { //positional parameter

			obj, ok := prop.Value.(*ast.ObjectLiteral)
			if !ok {
				onError(prop, "the description of a positional parameter should be an object")
				continue
			}

			missingPropertyNames := []string{"name", "pattern"}

			for _, paramDescProp := range obj.Properties {
				if paramDescProp.HasNoKey() {
					onError(paramDescProp, "the description of a positional parameter should not contain elements (values without a key)")
					continue
				}

				propName := paramDescProp.Name()

				for i, name := range missingPropertyNames {
					if name == propName {
						missingPropertyNames[i] = ""
					}
				}

				switch propName {
				case inoxconsts.MANIFEST_PARAM__DESCRIPTION_PROPNAME:
					switch paramDescProp.Value.(type) {
					case *ast.DoubleQuotedStringLiteral, *ast.MultilineStringLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be a string literal")
					}
				case inoxconsts.MANIFEST_POSITIONAL_PARAM__REST_PROPNAME:
					switch paramDescProp.Value.(type) {
					case *ast.BooleanLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be a string literal")
					}
				case inoxconsts.MANIFEST_NON_POSITIONAL_PARAM__NAME_PROPNAME:
					switch paramDescProp.Value.(type) {
					case *ast.UnambiguousIdentifierLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be an identifier (ex: #dir)")
					}
				case inoxconsts.MANIFEST_PARAM__PATTERN_PROPNAME:
					if !ast.NodeIsPattern(paramDescProp.Value) {
						onError(paramDescProp, "the .pattern of a positional parameter should be a named pattern or a pattern literal")
					}
				}
			}

			missingPropertyNames = utils.FilterSlice(missingPropertyNames, func(s string) bool { return s != "" })
			if len(missingPropertyNames) > 0 {
				onError(prop, "missing properties in description of positional parameter: "+strings.Join(missingPropertyNames, ", "))
			}
			//TODO: check unique rest parameter
			_ = obj
		}
	}
}
