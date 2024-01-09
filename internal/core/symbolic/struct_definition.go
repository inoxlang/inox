package symbolic

import (
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/parse"
	"golang.org/x/exp/slices"
)

func defineStructs(chunk *parse.ParsedChunk, statements []parse.Node, state *State) error {

	validDefinitions := predefineStructs(chunk, statements, state)
	comptimeTypes := state.symbolicData.GetCreateComptimeTypes(chunk.Node)

	//define members

	for _, item := range validDefinitions {
		structDef := item.definition
		structType := item.structType

		var (
			memberNames             []string
			methodNames             []string
			methodDeclarationsArray [32]*parse.FunctionDeclaration
			methodDeclarations      = methodDeclarationsArray[:0]

			//method dependency graph
			dependencyGraph memds.Graph32[string]
			//selfDependentArray [32]string
			//selfDependent      = selfDependentArray[:0]
		)

		//define fields and find method declarations

		for _, def := range structDef.Body.Definitions {
			switch def := def.(type) {
			case *parse.StructFieldDefinition:
				if slices.Contains(memberNames, def.Name.Name) {
					//ignore duplicate member definitions.
					continue
				}

				memberNames = append(memberNames, def.Name.Name)
				if err := handleStructFieldDefinition(def, structType, comptimeTypes, state); err != nil {
					return err
				}
			case *parse.FunctionDeclaration:
				methodName := def.Name.Name
				if slices.Contains(memberNames, methodName) {
					//ignore duplicate member definitions.
					continue
				}

				if len(methodDeclarations) >= 32 {
					state.addError(makeSymbolicEvalError(def, state, "too many methods (max 32)"))
					continue
				}

				memberNames = append(memberNames, methodName)
				methodNames = append(methodNames, methodName)

				dependencyGraph.AddNode(methodName)

				methodDeclarations = append(methodDeclarations, def)
			default:
				//invalid definition (parsing error)
			}
		}

		//define methods in two iterations

		//first iteration: check node types and build graph dependencies

		for _, decl := range methodDeclarations {
			//dependentKey := methodNames[i]
			//dependentKeyId, _ := dependencyGraph.IdOfNode(dependentKey)

			// find the method's dependencies
			parse.Walk(decl.Function, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

				if parse.IsScopeContainerNode(node) && node != decl.Function {
					return parse.Prune, nil
				}

				//checkNodeInStructMethodDefinition(node, ancestorChain, state)

				// selfExpr, ok := node.(*parse.SelfExpression)
				// if !ok {
				// 	return parse.ContinueTraversal, nil
				// }

				// dependencyName := ""

				// switch p := parent.(type) {
				// case *parse.MemberExpression:
				// 	dependencyName = p.PropertyName.Name
				// }

				// if dependencyName == "" {
				// 	return parse.ContinueTraversal, nil
				// }

				// depId, ok := dependencyGraph.IdOfNode(dependencyName)
				// if !ok {
				// 	//?
				// 	return parse.ContinueTraversal, nil
				// }

				// if dependentKeyId == depId {
				// 	selfDependent = append(selfDependent, dependentKey)
				// } else if !dependencyGraph.HasEdgeFromTo(dependentKeyId, depId) {
				// 	// dependentKey ->- depKey
				// 	dependencyGraph.AddEdge(dependentKeyId, depId)
				// }

				return parse.ContinueTraversal, nil
			}, nil)
		}

		//second iteration

		for _, decl := range methodDeclarations {
			if err := handleStructMethodDefinition(decl, structType, comptimeTypes, state); err != nil {
				return err
			}
		}
	}

	return nil
}

type validStructDefinition struct {
	definition *parse.StructDefinition
	structType *StructType
}

func predefineStructs(chunk *parse.ParsedChunk, statements []parse.Node, state *State) (defs []validStructDefinition) {
	structDefs := &[]validStructDefinition{}
	comptimeTypes := state.symbolicData.GetCreateComptimeTypes(chunk.Node)

	_predefineStructs(chunk, statements, state, structDefs, comptimeTypes)
	return *structDefs
}

func _predefineStructs(
	chunk *parse.ParsedChunk,
	statements []parse.Node,
	state *State,

	defs *[]validStructDefinition,
	comptimeTypes *ModuleCompileTimeTypes,
) {

	for _, stmt := range statements {
		inclusionImport, ok := stmt.(*parse.InclusionImportStatement)
		if !ok || parse.HasErrorAtAnyDepth(inclusionImport) {
			continue
		}

		includedChunk, ok := state.Module.inclusionStatementMap[inclusionImport]
		if !ok {
			continue
		}
		_predefineStructs(includedChunk.ParsedChunk, includedChunk.Node.Statements, state, defs, comptimeTypes)
	}

	for _, stmt := range statements {
		structDef, ok := stmt.(*parse.StructDefinition)
		if !ok || structDef.Name == nil {
			continue
		}

		ident, ok := structDef.Name.(*parse.PatternIdentifierLiteral)
		if !ok {
			continue
		}
		name := ident.Name

		if comptimeTypes.IsTypeDefined(name) {
			//duplicate definition (static check error)
		}

		structType := &StructType{
			name: name,
		}

		//save the stuct type
		comptimeTypes.DefineType(name, structType)

		if structDef.Body == nil {
			continue
		}

		*defs = append(*defs, validStructDefinition{
			definition: structDef,
			structType: structType,
		})
	}
}

func handleStructFieldDefinition(
	def *parse.StructFieldDefinition,
	structType *StructType,
	comptimeTypes *ModuleCompileTimeTypes,
	state *State,
) error {
	if def.Type == nil {
		return nil
	}

	var fieldType CompileTimeType

	switch typeNode := def.Type.(type) {
	case *parse.PatternIdentifierLiteral:
		typeName := typeNode.Name
		patt := state.ctx.ResolveNamedPattern(typeName)
		if patt != nil && !IsNameOfBuiltinComptimeType(typeName) {
			state.addError(makeSymbolicEvalError(typeNode, state, ONLY_COMPILE_TIME_TYPES_CAN_BE_USED_AS_STRUCT_FIELD_TYPES))
			return nil
		}

		comptimeType, ok := comptimeTypes.GetType(typeName)
		if !ok {
			state.addError(makeSymbolicEvalError(typeNode, state, fmtCompileTimeTypeIsNotDefined(typeName)))
			return nil
		}

		fieldType = comptimeType
	case *parse.PointerType:
		state.addError(makeSymbolicEvalError(typeNode, state, "pointer types are not supported yet"))
		return nil

		// patternIdent, ok := typeNode.ValueType.(*parse.PatternIdentifierLiteral)
		// if !ok {
		// 	//static check error
		// 	return nil
		// }
		// ptrType, ok := comptimeTypes.GetPointerType(patternIdent.Name)
		// if !ok {
		// 	state.addError(makeSymbolicEvalError(typeNode, state, fmtCompileTimeTypeIsNotDefined(patternIdent.Name)))
		// 	return nil
		// }
		// fieldType = ptrType
	// case *parse.PatternCallExpression: //TODO: support integers in a given range
	default:
		state.addError(makeSymbolicEvalError(typeNode, state, ONLY_COMPILE_TIME_TYPES_CAN_BE_USED_AS_STRUCT_FIELD_TYPES))
	}

	structType.fields = append(structType.fields, StructField{
		Name: def.Name.Name,
		Type: fieldType,
	})

	return nil
}

func handleStructMethodDefinition(
	def *parse.FunctionDeclaration,
	structType *StructType,
	comptimeTypes *ModuleCompileTimeTypes,
	state *State,
) error {

	if def.Err != nil {
		return nil
	}

	name := def.Name.Name

	//TODO: support recursive methods and

	v, err := symbolicEval(def.Function, state)
	if err != nil {
		return err
	}
	state.symbolicData.SetMostSpecificNodeValue(def.Name, v)

	structType.methods = append(structType.methods, StructMethod{
		Name:  name,
		Value: v.(*InoxFunction),
	})

	return nil
}

// func checkNodeInStructMethodDefinition(node parse.Node, ancestorChain []parse.Node, state *State) error {

// 	switch n := node.(type) {
// 	case
// 		//variables
// 		*parse.IdentifierLiteral, *parse.Variable,

// 		//declarations
// 		*parse.LocalVariableDeclarations, *parse.LocalVariableDeclaration,
// 		*parse.GlobalVariableDeclarations, *parse.GlobalVariableDeclaration,

// 		//assignment
// 		*parse.Assignment, *parse.MultiAssignment,

// 		//types
// 		*parse.PatternIdentifierLiteral, *parse.PointerType,

// 		//operations
// 		*parse.BinaryExpression:

// 	case *parse.CallExpression:
// 		allowed := false

// 		ident, ok := n.Callee.(*parse.IdentifierLiteral)
// 		if ok {
// 			switch ident.Name {
// 			case globalnames.LEN_FN:
// 				allowed = true
// 			}
// 		}

// 		if !allowed {
// 			c.addError(n, SYNTAX_ELEM_NOT_ALLOWED_INSIDE_STRUCT_DEFS)
// 		}
// 	default:
// 		if !parse.NodeIsSimpleValueLiteral(n) {
// 			c.addError(n, fmtFollowingNodeTypeNotAllowedInAssertions(n))
// 		}
// 	}

// }
