package core

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

type preinitBlockCheckParams struct {
	node    *parse.PreinitStatement
	fls     afs.Filesystem
	onError func(n parse.Node, msg string)
	module  *Module
}

func checkPreinitBlock(args preinitBlockCheckParams) {
	parse.Walk(args.node.Block, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		case *parse.Block, *parse.IdentifierLiteral, *parse.GlobalVariable,
			parse.SimpleValueLiteral, *parse.URLExpression,
			*parse.IntegerRangeLiteral, *parse.FloatRangeLiteral,

			//patterns
			*parse.PatternDefinition, *parse.PatternIdentifierLiteral,
			*parse.PatternNamespaceDefinition, *parse.PatternConversionExpression,
			*parse.ComplexStringPatternPiece, *parse.PatternPieceElement,
			*parse.ObjectPatternLiteral, *parse.RecordPatternLiteral, *parse.ObjectPatternProperty,
			*parse.PatternCallExpression, *parse.PatternGroupName,
			*parse.PatternUnion, *parse.ListPatternLiteral, *parse.TuplePatternLiteral:

			//ok
		case *parse.InclusionImportStatement:
			includedChunk := args.module.InclusionStatementMap[n]

			checkPatternOnlyIncludedChunk(includedChunk.Node, args.onError)
		default:
			args.onError(n, fmt.Sprintf("%s: %T", ErrForbiddenNodeinPreinit, n))
			return parse.Prune, nil
		}

		return parse.ContinueTraversal, nil
	}, nil)
}

func checkPatternOnlyIncludedChunk(chunk *parse.Chunk, onError func(n parse.Node, msg string)) {
	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		if node == chunk {
			return parse.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case *parse.IncludableChunkDescription,
			parse.SimpleValueLiteral, *parse.URLExpression,
			*parse.IntegerRangeLiteral, *parse.FloatRangeLiteral,

			//patterns
			*parse.PatternDefinition, *parse.PatternIdentifierLiteral,
			*parse.PatternNamespaceDefinition, *parse.PatternConversionExpression,
			*parse.ComplexStringPatternPiece, *parse.PatternPieceElement,
			*parse.ObjectPatternLiteral, *parse.RecordPatternLiteral, *parse.ObjectPatternProperty,
			*parse.PatternCallExpression, *parse.PatternGroupName,
			*parse.PatternUnion, *parse.ListPatternLiteral, *parse.TuplePatternLiteral:
		default:
			onError(n, fmt.Sprintf("%s: %T", text.FORBIDDEN_NODE_TYPE_IN_INCLUDABLE_CHUNK_IMPORTED_BY_PREINIT, n))
			return parse.Prune, nil
		}

		return parse.ContinueTraversal, nil
	}, nil)
}

type manifestStaticCheckArguments struct {
	objLit                *parse.ObjectLiteral
	ignoreUnknownSections bool
	moduleKind            ModuleKind
	onError               func(n parse.Node, msg string)
	project               Project
}

func checkManifestObject(args manifestStaticCheckArguments) {
	objLit := args.objLit
	ignoreUnknownSections := args.ignoreUnknownSections
	onError := args.onError

	parse.Walk(objLit, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		case *parse.ObjectLiteral:
			if len(n.SpreadElements) != 0 {
				onError(n, text.NO_SPREAD_IN_MANIFEST)
			}
			shallowCheckObjectRecordProperties(n.Properties, nil, true, func(n parse.Node, msg string) {
				onError(n, msg)
			})
		case *parse.RecordLiteral:
			if len(n.SpreadElements) != 0 {
				onError(n, text.NO_SPREAD_IN_MANIFEST)
			}
			shallowCheckObjectRecordProperties(n.Properties, nil, false, func(n parse.Node, msg string) {
				onError(n, msg)
			})
		case *parse.ListLiteral:
			if n.HasSpreadElements() {
				onError(n, text.NO_SPREAD_IN_MANIFEST)
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)

	for _, p := range objLit.Properties {
		if p.HasImplicitKey() {
			onError(p, text.ELEMENTS_NOT_ALLOWED_IN_MANIFEST)
			continue
		}

		sectionName := p.Name()
		allowedSectionNames := MODULE_KIND_TO_ALLOWED_SECTION_NAMES[args.moduleKind]
		if !slices.Contains(allowedSectionNames, sectionName) {
			onError(p.Key, text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind(sectionName, args.moduleKind))
			continue
		}

		switch sectionName {
		case inoxconsts.MANIFEST_KIND_SECTION_NAME:
			kindName, ok := getUncheckedModuleKindNameFromNode(p.Value)
			if !ok {
				onError(p.Key, text.KIND_SECTION_SHOULD_BE_A_STRING_LITERAL)
				continue
			}

			kind, err := ParseModuleKind(kindName)
			if err != nil {
				onError(p.Key, ErrInvalidModuleKind.Error())
				continue
			}
			if kind.IsEmbedded() {
				onError(p.Key, text.INVALID_KIND_SECTION_EMBEDDED_MOD_KINDS_NOT_ALLOWED)
				continue
			}
		case inoxconsts.MANIFEST_PERMS_SECTION_NAME:
			if obj, ok := p.Value.(*parse.ObjectLiteral); ok {
				checkPermissionListingObject(obj, onError)
			} else {
				onError(p, text.PERMS_SECTION_SHOULD_BE_AN_OBJECT)
			}
		case inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME:
			dict, ok := p.Value.(*parse.DictionaryLiteral)
			if !ok {
				onError(p, text.HOST_DEFS_SECTION_SHOULD_BE_A_DICT)
				continue
			}

			hasErrors := false

			parse.Walk(dict, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if node == dict {
					return parse.ContinueTraversal, nil
				}

				switch n := node.(type) {
				case *parse.ObjectLiteral, *parse.ObjectProperty:
				case *parse.DictionaryEntry, parse.SimpleValueLiteral, *parse.GlobalVariable,
					*parse.IdentifierMemberExpression:
				default:
					hasErrors = true
					onError(n, text.FmtForbiddenNodeInHostDefinitionsSection(n))
				}

				return parse.ContinueTraversal, nil
			}, nil)

			if !hasErrors {
				staticallyCheckHostDefinitionFnRegistryLock.Lock()
				defer staticallyCheckHostDefinitionFnRegistryLock.Unlock()

				for _, entry := range dict.Entries {
					key := entry.Key

					switch k := key.(type) {
					case *parse.InvalidURL:
					case *parse.HostLiteral:
						host := utils.Must(EvalSimpleValueLiteral(k, nil)).(Host)
						fn, ok := staticallyCheckHostDefinitionDataFnRegistry[host.Scheme()]
						if ok {
							errMsg := fn(args.project, entry.Value)
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
			obj, ok := p.Value.(*parse.ObjectLiteral)

			if !ok {
				onError(p, text.LIMITS_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			parse.Walk(obj, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if node == obj {
					return parse.ContinueTraversal, nil
				}

				switch n := node.(type) {
				case *parse.ObjectProperty, parse.SimpleValueLiteral, *parse.GlobalVariable:
				default:
					onError(n, text.FmtForbiddenNodeInLimitsSection(n))
				}

				return parse.ContinueTraversal, nil
			}, nil)
		case inoxconsts.MANIFEST_ENV_SECTION_NAME:

			if args.moduleKind.IsEmbedded() {
				onError(p, text.ENV_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			patt, ok := p.Value.(*parse.ObjectPatternLiteral)

			if !ok {
				onError(p, text.ENV_SECTION_SHOULD_BE_AN_OBJECT_PATTERN)
				continue
			}

			parse.Walk(patt, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if node == patt {
					return parse.ContinueTraversal, nil
				}

				switch n := node.(type) {
				case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
					*parse.ObjectPatternProperty, *parse.PatternCallExpression, parse.SimpleValueLiteral, *parse.GlobalVariable:
				default:
					onError(n, text.FmtForbiddenNodeInEnvSection(n))
				}

				return parse.ContinueTraversal, nil
			}, nil)
		case inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME:
			if args.moduleKind.IsEmbedded() {
				onError(p, text.PREINIT_FILES_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			obj, ok := p.Value.(*parse.ObjectLiteral)

			if !ok {
				onError(p, text.PREINIT_FILES_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			checkPreinitFilesObject(obj, onError)
		case inoxconsts.MANIFEST_DATABASES_SECTION_NAME:
			if args.moduleKind.IsEmbedded() {
				onError(p, text.DATABASES_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			switch propVal := p.Value.(type) {
			case *parse.ObjectLiteral:
				checkDatabasesObject(propVal, onError, nil, args.project)
			case *parse.AbsolutePathLiteral:
			default:
				onError(p, text.DATABASES_SECTION_SHOULD_BE_AN_OBJECT_OR_ABS_PATH)
			}
		case inoxconsts.MANIFEST_INVOCATION_SECTION_NAME:
			if args.moduleKind.IsEmbedded() {
				onError(p, text.INVOCATION_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			switch propVal := p.Value.(type) {
			case *parse.ObjectLiteral:
				checkInvocationObject(propVal, objLit, onError, args.project)
			default:
				onError(p, text.INVOCATION_SECTION_SHOULD_BE_AN_OBJECT)
			}
		case inoxconsts.MANIFEST_PARAMS_SECTION_NAME:
			if args.moduleKind.IsEmbedded() {
				onError(p, text.PARAMS_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS)
				continue
			}

			obj, ok := p.Value.(*parse.ObjectLiteral)

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

func checkPermissionListingObject(objLit *parse.ObjectLiteral, onError func(n parse.Node, msg string)) {
	parse.Walk(objLit, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		case *parse.ObjectLiteral, *parse.ListLiteral, *parse.DictionaryLiteral, *parse.DictionaryEntry, *parse.ObjectProperty,
			parse.SimpleValueLiteral, *parse.GlobalVariable, *parse.PatternIdentifierLiteral, *parse.URLExpression, *parse.PathPatternExpression:
		default:
			onError(n, text.FmtForbiddenNodeInPermListing(n))
		}

		return parse.ContinueTraversal, nil
	}, nil)

	for _, p := range objLit.Properties {
		if p.HasImplicitKey() {
			onError(p, text.ELEMENTS_NOT_ALLOWED_IN_PERMS_SECTION)
			continue
		}

		propName := p.Name()
		permKind, ok := permkind.PermissionKindFromString(propName)
		if !ok {
			onError(p.Key, text.FmtNotValidPermissionKindName(p.Name()))
			continue
		}
		checkSingleKindPermissions(permKind, p.Value, onError)
	}
}

func checkSingleKindPermissions(permKind PermissionKind, desc parse.Node, onError func(n parse.Node, msg string)) {
	checkSingleItem := func(node parse.Node) {
		switch n := node.(type) {
		case *parse.AbsolutePathExpression:
		case *parse.AbsolutePathLiteral:
		case *parse.RelativePathLiteral:
			onError(n, text.FmtOnlyAbsPathsAreAcceptedInPerms(n.Raw))
		case *parse.AbsolutePathPatternLiteral:
		case *parse.RelativePathPatternLiteral:
			onError(n, text.FmtOnlyAbsPathPatternsAreAcceptedInPerms(n.Raw))
		case *parse.URLExpression:
		case *parse.URLLiteral:
		case *parse.URLPatternLiteral:
		case *parse.HostLiteral:
		case *parse.HostPatternLiteral:
		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceIdentifierLiteral:
		case *parse.GlobalVariable, *parse.Variable, *parse.IdentifierLiteral:

		case *parse.DoubleQuotedStringLiteral, *parse.MultilineStringLiteral, *parse.UnquotedStringLiteral:
			s := n.(parse.SimpleValueLiteral).ValueString()

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
	case *parse.ListLiteral:
		for _, elem := range v.Elements {
			checkSingleItem(elem)
		}
	case *parse.ObjectLiteral:
		for _, prop := range v.Properties {
			if prop.HasImplicitKey() {
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

func checkPreinitFilesObject(obj *parse.ObjectLiteral, onError func(n parse.Node, msg string)) {

	hasForbiddenNodes := false

	parse.Walk(obj, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		if node == obj {
			return parse.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression, *parse.ObjectLiteral,
			*parse.ObjectProperty, *parse.PatternCallExpression, parse.SimpleValueLiteral, *parse.GlobalVariable,
			*parse.AbsolutePathExpression, *parse.RelativePathExpression:
		default:
			onError(n, text.FmtForbiddenNodeInPreinitFilesSection(n))
			hasForbiddenNodes = true
		}

		return parse.ContinueTraversal, nil
	}, nil)

	if hasForbiddenNodes {
		return
	}

	for _, p := range obj.Properties {
		if p.Value == nil {
			continue
		}
		fileDesc, ok := p.Value.(*parse.ObjectLiteral)
		if !ok {
			onError(p.Value, text.PREINIT_FILES__FILE_CONFIG_SHOULD_BE_AN_OBJECT)
			continue
		}

		pathNode, ok := fileDesc.PropValue(inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME)

		if !ok {
			onError(p, text.FmtMissingPropInPreinitFileDescription(inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME, p.Name()))
		} else {
			switch pathNode.(type) {
			case *parse.AbsolutePathLiteral, *parse.AbsolutePathExpression:
			default:
				onError(p, text.PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH)
			}
		}

		if !fileDesc.HasNamedProp(inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME) {
			onError(p, text.FmtMissingPropInPreinitFileDescription(inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME, p.Name()))
		}

	}
}

func checkDatabasesObject(
	obj *parse.ObjectLiteral,
	onError func(n parse.Node, msg string), //optional
	onValidDatabase func(name string, scheme Scheme, resource ResourceName), //optional
	project Project,
) {

	if onError == nil {
		onError = func(n parse.Node, msg string) {}
	}

	if onValidDatabase == nil {
		onValidDatabase = func(name string, scheme Scheme, resource ResourceName) {}
	}

	parse.Walk(obj, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		if node == obj {
			return parse.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression, *parse.ObjectLiteral,
			*parse.ObjectProperty, *parse.PatternCallExpression, parse.SimpleValueLiteral, *parse.GlobalVariable,
			*parse.AbsolutePathExpression, *parse.RelativePathExpression:
		default:
			onError(n, text.FmtForbiddenNodeInDatabasesSection(n))
		}

		return parse.ContinueTraversal, nil
	}, nil)

	for _, p := range obj.Properties {
		if p.HasImplicitKey() || p.Value == nil {
			continue
		}
		dbName := p.Name()

		dbDesc, ok := p.Value.(*parse.ObjectLiteral)
		if !ok {
			onError(p.Value, text.DATABASES__DB_CONFIG_SHOULD_BE_AN_OBJECT)
			continue
		}

		var scheme Scheme
		var resource ResourceName
		var resourceFound bool
		var resolutionDataFound bool
		isValidDescription := true

		for _, prop := range dbDesc.Properties {
			if prop.HasImplicitKey() {
				continue
			}

			switch prop.Name() {
			case inoxconsts.MANIFEST_DATABASE__RESOURCE_PROP_NAME:
				resourceFound = true

				switch res := prop.Value.(type) {
				case *parse.HostLiteral:
					u, _ := url.Parse(res.Value)
					if u != nil {
						scheme = Scheme(u.Scheme)
						resource = utils.Must(EvalSimpleValueLiteral(res, nil)).(Host)
					}
				case *parse.URLLiteral:
					u, _ := url.Parse(res.Value)
					if u != nil {
						scheme = Scheme(u.Scheme)
						resource = utils.Must(EvalSimpleValueLiteral(res, nil)).(URL)
					}
				default:
					isValidDescription = false
					onError(p, text.DATABASES__DB_RESOURCE_SHOULD_BE_HOST_OR_URL)
				}
			case inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME:
				resolutionDataFound = true

				switch prop.Value.(type) {
				case *parse.NilLiteral, *parse.HostLiteral, *parse.RelativePathLiteral, *parse.AbsolutePathLiteral,
					*parse.AbsolutePathExpression, *parse.RelativePathExpression:
					if scheme == "" {
						break
					}
					checkData, ok := GetStaticallyCheckDbResolutionDataFn(scheme)
					if ok {
						errMsg := checkData(prop.Value, project)
						if errMsg != "" {
							isValidDescription = false
							onError(prop.Value, errMsg)
						}
					}
				default:
					isValidDescription = false
					onError(p, text.DATABASES__DB_RESOLUTION_DATA_ONLY_NIL_AND_PATHS_SUPPORTED)
				}
			case inoxconsts.MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME:
				switch prop.Value.(type) {
				case *parse.BooleanLiteral:
				default:
					isValidDescription = false
					onError(p, text.DATABASES__DB_EXPECTED_SCHEMA_UPDATE_SHOULD_BE_BOOL_LIT)
				}
			case inoxconsts.MANIFEST_DATABASE__ASSERT_SCHEMA_UPDATE_PROP_NAME:
				switch prop.Value.(type) {
				case *parse.PatternIdentifierLiteral, *parse.ObjectPatternLiteral:
				default:
					isValidDescription = false
					onError(p, text.DATABASES__DB_ASSERT_SCHEMA_SHOULD_BE_PATT_IDENT_OR_OBJ_PATT)
				}
			default:
				isValidDescription = false
				onError(p, text.FmtUnexpectedPropOfDatabaseDescription(prop.Name()))
			}
		}

		if !resourceFound {
			onError(p, text.FmtMissingPropInDatabaseDescription(inoxconsts.MANIFEST_DATABASE__RESOURCE_PROP_NAME, dbName))
		}

		if !resolutionDataFound {
			onError(p, text.FmtMissingPropInDatabaseDescription(inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME, dbName))
		}

		if isValidDescription {
			onValidDatabase(dbName, scheme, resource)
		}
	}
}

func checkInvocationObject(obj *parse.ObjectLiteral, manifestObj *parse.ObjectLiteral, onError func(n parse.Node, msg string), project Project) {

	for _, p := range obj.Properties {
		if p.Value == nil {
			continue
		}

		if p.HasImplicitKey() {
			continue
		}

		switch p.Name() {
		case inoxconsts.MANIFEST_INVOCATION__ON_ADDED_ELEM_PROP_NAME:
			if urlLit, ok := p.Value.(*parse.URLLiteral); ok {
				scheme, err := urlLit.Scheme()

				if err == nil {
					if !IsStaticallyCheckDBFunctionRegistered(Scheme(scheme)) {
						onError(manifestObj, text.SCHEME_NOT_DB_SCHEME_OR_IS_NOT_SUPPORTED)
					} else {
						//if the scheme corresponds to a database and the manifest does not
						//contain the databases section, we add an error
						if !manifestObj.HasNamedProp(inoxconsts.MANIFEST_DATABASES_SECTION_NAME) {
							onError(manifestObj, text.THE_DATABASES_SECTION_SHOULD_BE_PRESENT)
						}
					}
				}

			} else {
				onError(p.Value, text.ONLY_URL_LITS_ARE_SUPPORTED_FOR_NOW)
			}
		case inoxconsts.MANIFEST_INVOCATION__ASYNC_PROP_NAME:
			if _, ok := p.Value.(*parse.BooleanLiteral); !ok {
				onError(p.Value, text.A_BOOL_LIT_IS_EXPECTED)
			}
		default:
			onError(p, text.FmtUnexpectedPropOfInvocationDescription(p.Name()))
		}
	}
}

func checkParametersObject(objLit *parse.ObjectLiteral, onError func(n parse.Node, msg string)) {

	parse.Walk(objLit, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		if node == objLit {
			return parse.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case
			*parse.ObjectProperty, *parse.ObjectLiteral, *parse.ListLiteral,
			*parse.OptionExpression,
			parse.SimpleValueLiteral, *parse.GlobalVariable,
			//patterns
			*parse.PatternCallExpression,
			*parse.ListPatternLiteral, *parse.TuplePatternLiteral,
			*parse.ObjectPatternLiteral, *parse.ObjectPatternProperty, *parse.RecordPatternLiteral,
			*parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression, *parse.PatternNamespaceIdentifierLiteral,
			*parse.PatternConversionExpression,
			*parse.PatternUnion,
			*parse.PathPatternExpression, *parse.AbsolutePathPatternLiteral, *parse.RelativePathPatternLiteral,
			*parse.URLPatternLiteral, *parse.HostPatternLiteral, *parse.OptionalPatternExpression,
			*parse.OptionPatternLiteral, *parse.FunctionPatternExpression, *parse.NamedSegmentPathPatternLiteral:
		default:
			onError(n, text.FmtForbiddenNodeInParametersSection(n))
		}

		return parse.ContinueTraversal, nil
	}, nil)

	positionalParamsEnd := false

	for _, prop := range objLit.Properties {
		if !prop.HasImplicitKey() { // non positional parameter
			positionalParamsEnd = true

			propValue := prop.Value
			optionPattern, isOptionPattern := prop.Value.(*parse.OptionPatternLiteral)
			if isOptionPattern {
				propValue = optionPattern.Value
			}

			switch propVal := propValue.(type) {
			case *parse.ObjectLiteral:
				if isOptionPattern {
					break
				}

				missingPropertyNames := []string{"pattern"}

				for _, paramDescProp := range propVal.Properties {
					if paramDescProp.HasImplicitKey() {
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
						if !parse.NodeIsPattern(paramDescProp.Value) {
							onError(paramDescProp, "the .pattern of a non positional parameter should be a named pattern or a pattern literal")
						}
					case "default":
					case "char-name":
						switch paramDescProp.Value.(type) {
						case *parse.RuneLiteral:
						default:
							onError(paramDescProp, "the .char-name of a non positional parameter should be a rune literal")
						}
					case "description":
						switch paramDescProp.Value.(type) {
						case *parse.DoubleQuotedStringLiteral, *parse.MultilineStringLiteral:
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
				if !parse.NodeIsPattern(prop.Value) {
					onError(prop, "the description of a non positional parameter should be a named pattern or a pattern literal")
				}
			}

		} else if positionalParamsEnd {
			onError(prop, "elements (values with no key) describe positional parameters, all implict key properties should be at the top of the 'parameters' section")
		} else { //positional parameter

			obj, ok := prop.Value.(*parse.ObjectLiteral)
			if !ok {
				onError(prop, "the description of a positional parameter should be an object")
				continue
			}

			missingPropertyNames := []string{"name", "pattern"}

			for _, paramDescProp := range obj.Properties {
				if paramDescProp.HasImplicitKey() {
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
					case *parse.DoubleQuotedStringLiteral, *parse.MultilineStringLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be a string literal")
					}
				case inoxconsts.MANIFEST_POSITIONAL_PARAM__REST_PROPNAME:
					switch paramDescProp.Value.(type) {
					case *parse.BooleanLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be a string literal")
					}
				case inoxconsts.MANIFEST_NON_POSITIONAL_PARAM__NAME_PROPNAME:
					switch paramDescProp.Value.(type) {
					case *parse.UnambiguousIdentifierLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be an identifier (ex: #dir)")
					}
				case inoxconsts.MANIFEST_PARAM__PATTERN_PROPNAME:
					if !parse.NodeIsPattern(paramDescProp.Value) {
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
