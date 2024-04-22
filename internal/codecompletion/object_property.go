package codecompletion

import (
	"encoding/json"
	"strings"

	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func findObjectInteriorCompletions(n *parse.ObjectLiteral, search completionSearch) (completions []Completion) {
	chunk := search.chunk
	cursorIndex := int32(search.cursorIndex)
	ancestors := search.ancestorChain

	interiorSpan, err := parse.GetInteriorSpan(n, chunk.Node)
	if err != nil {
		return nil
	}

	if !interiorSpan.HasPositionEndIncluded(cursorIndex) {
		return nil
	}

	pos := chunk.GetSourcePosition(parse.NodeSpan{Start: cursorIndex, End: cursorIndex})

	properties, ok := search.state.Global.SymbolicData.GetAllowedNonPresentProperties(n)
	if ok {
		for _, name := range properties {
			propNameAndColon := quotePropNameIfNecessary(name) + ": "
			completions = append(completions, Completion{
				ShownString:   propNameAndColon,
				Value:         propNameAndColon,
				Kind:          defines.CompletionItemKindProperty,
				ReplacedRange: pos,
			})
		}
	}

	switch parent := search.parent.(type) {
	case *parse.Manifest: //suggest sections of the manifest that are not present
	manifest_sections_loop:
		for _, sectionName := range inoxconsts.MANIFEST_SECTION_NAMES {
			for _, prop := range n.Properties {
				if !prop.HasNoKey() && prop.Name() == sectionName {
					continue manifest_sections_loop
				}
			}

			suffix := ": "
			valueCompletion, ok := MANIFEST_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
			if ok {
				suffix += valueCompletion
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				MarkdownDocumentation: MANIFEST_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
				ReplacedRange:         pos,
			})
		}
	case *parse.ImportStatement: //suggest sections of the module import config that are not present
	mod_import_sections_loop:
		for _, sectionName := range inoxconsts.IMPORT_CONFIG_SECTION_NAMES {
			for _, prop := range n.Properties {
				if !prop.HasNoKey() && prop.Name() == sectionName {
					continue mod_import_sections_loop
				}
			}

			suffix := ": "
			valueCompletion, ok := MODULE_IMPORT_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
			if ok {
				suffix += valueCompletion
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				MarkdownDocumentation: MODULE_IMPORT_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
				ReplacedRange:         pos,
			})
		}
	case *parse.SpawnExpression:
		if n != parent.Meta {
			break
		}
		//suggest sections of the lthread meta object that are not present
	lthread_meta_sections_loop:
		for _, sectionName := range symbolic.LTHREAD_SECTION_NAMES {
			for _, prop := range n.Properties {
				if !prop.HasNoKey() && prop.Name() == sectionName {
					continue lthread_meta_sections_loop
				}
			}

			suffix := ": "
			valueCompletion, ok := LTHREAD_META_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
			if ok {
				suffix += valueCompletion
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				LabelDetail:           LTHREAD_META_SECTION_LABEL_DETAILS[sectionName],
				MarkdownDocumentation: LTHREAD_META_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
				ReplacedRange:         pos,
			})
		}
	case *parse.ObjectProperty:
		if parent.HasNoKey() || len(ancestors) < 3 {
			return
		}

		//allowed permissions in module import statement
		if len(ancestors) >= 5 &&
			parent.HasNameEqualTo(inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME) &&
			utils.Implements[*parse.ImportStatement](ancestors[len(ancestors)-3]) {

			for _, info := range permbase.PERMISSION_KINDS {
				//ignore kinds that are already present.
				if n.HasNamedProp(info.Name) {
					continue
				}

				detail := MAJOR_PERM_KIND_TEXT

				if info.PermissionKind.IsMinor() {
					detail = MINOR_PERM_KIND_TEXT
				}

				completions = append(completions, Completion{
					ShownString:   info.Name,
					Value:         info.Name,
					Kind:          defines.CompletionItemKindVariable,
					ReplacedRange: pos,
					LabelDetail:   detail,
				})
			}
		}

		switch greatGrandParent := ancestors[len(ancestors)-3].(type) {
		case *parse.Manifest:
			switch parent.Name() {
			case inoxconsts.MANIFEST_PERMS_SECTION_NAME: //permissions section
				for _, info := range permbase.PERMISSION_KINDS {
					//ignore kinds that are already present.
					if n.HasNamedProp(info.Name) {
						continue
					}

					detail := MAJOR_PERM_KIND_TEXT

					if info.PermissionKind.IsMinor() {
						detail = MINOR_PERM_KIND_TEXT
					}

					completions = append(completions, Completion{
						ShownString:   info.Name,
						Value:         info.Name,
						Kind:          defines.CompletionItemKindVariable,
						ReplacedRange: pos,
						LabelDetail:   detail,
					})
				}
			}
		default:
			_ = greatGrandParent
		}

		if len(ancestors) < 5 {
			break
		}

		manifestSectionName := ""
		var sectionProperty *parse.ObjectProperty

		ancestorCount := len(ancestors)

		if utils.Implements[*parse.Manifest](ancestors[ancestorCount-5]) &&
			utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-4]) &&
			utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-3]) &&
			ancestors[ancestorCount-3].(*parse.ObjectProperty).Key != nil {
			sectionProperty = ancestors[ancestorCount-3].(*parse.ObjectProperty)
			manifestSectionName = sectionProperty.Name()
		}

		if sectionProperty == nil || sectionProperty.Value == nil {
			break
		}

		//the cursor is located in the span of an object inside a manifest section.

		switch manifestSectionName {
		case inoxconsts.MANIFEST_DATABASES_SECTION_NAME:
			//suggest database description's properties

			_, ok := sectionProperty.Value.(*parse.ObjectLiteral)
			if !ok {
				break
			}
			dbDescription := n

			for _, descPropName := range inoxconsts.MANIFEST_DATABASE_PROPNAMES {
				//ignore properties that are already present.
				if dbDescription.HasNamedProp(descPropName) {
					continue
				}

				suffix := ": "
				valueCompletion, ok := MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS[descPropName]
				if ok {
					suffix += valueCompletion
				}

				completions = append(completions, Completion{
					ShownString:           descPropName + suffix,
					Value:                 descPropName + suffix,
					Kind:                  defines.CompletionItemKindVariable,
					MarkdownDocumentation: MANIFEST_DB_DESC_DOC[descPropName],
					ReplacedRange:         pos,
				})
			}
		}
	}

	return
}

func findRecordInteriorCompletions(n *parse.RecordLiteral, search completionSearch) (completions []Completion) {
	cursorIndex := int32(search.cursorIndex)
	chunk := search.chunk

	interiorSpan, err := parse.GetInteriorSpan(n, chunk.Node)
	if err != nil {
		return nil
	}

	if !interiorSpan.HasPositionEndIncluded(cursorIndex) {
		return nil
	}

	pos := chunk.GetSourcePosition(parse.NodeSpan{Start: cursorIndex, End: cursorIndex})

	properties, ok := search.state.Global.SymbolicData.GetAllowedNonPresentProperties(n)
	if ok {
		for _, name := range properties {
			propNameAndColon := quotePropNameIfNecessary(name) + ": "
			completions = append(completions, Completion{
				ShownString:   propNameAndColon,
				Value:         propNameAndColon,
				Kind:          defines.CompletionItemKindProperty,
				ReplacedRange: pos,
			})
		}
	}
	return
}

func findObjectPropertyNameCompletions(
	ident *parse.IdentifierLiteral,
	prop *parse.ObjectProperty,
	ancestors []parse.Node,
	search completionSearch,
) (completions []Completion) {
	ancestorCount := len(ancestors)
	objectLiteral := ancestors[ancestorCount-2].(*parse.ObjectLiteral)

	//suggest sections of manifest
	if utils.Implements[*parse.Manifest](ancestors[ancestorCount-3]) {
		manifestObject := objectLiteral

		for _, sectionName := range inoxconsts.MANIFEST_SECTION_NAMES {
			if manifestObject.HasNamedProp(sectionName) {
				//ignore properties that are already present.
				continue
			}

			if !hasPrefixCaseInsensitive(sectionName, ident.Name) {
				//ignore properties that don't have ident.Name as prefix.
				continue
			}

			suffix := ""
			if prop.HasNoKey() {
				suffix = ": "

				valueCompletion, ok := MANIFEST_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
				if ok {
					suffix += valueCompletion
				}
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				MarkdownDocumentation: MANIFEST_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
			})
		}
		return completions
	}

	//suggest properties of database descriptions
	if ancestorCount >= 7 && utils.Implements[*parse.Manifest](ancestors[ancestorCount-7]) &&
		utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-6]) &&
		utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-5]) &&
		ancestors[ancestorCount-5].(*parse.ObjectProperty).HasNameEqualTo(inoxconsts.MANIFEST_DATABASES_SECTION_NAME) &&
		utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-4]) &&
		utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-3]) &&
		utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-2]) {

		dbDesc := objectLiteral

		for _, descPropName := range inoxconsts.MANIFEST_DATABASE_PROPNAMES {
			if dbDesc.HasNamedProp(descPropName) {
				//ignore properties that are already present.
				continue
			}

			if !hasPrefixCaseInsensitive(descPropName, ident.Name) {
				//ignore properties that don't have ident.Name as prefix.
				continue
			}

			suffix := ""
			if prop.HasNoKey() {
				suffix = ": "

				valueCompletion, ok := MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS[descPropName]
				if ok {
					suffix += valueCompletion
				}
			}

			completions = append(completions, Completion{
				ShownString:           descPropName + suffix,
				Value:                 descPropName + suffix,
				Kind:                  defines.CompletionItemKindVariable,
				MarkdownDocumentation: MANIFEST_DB_DESC_DOC[descPropName],
			})
		}
		return completions
	}

	//suggest sections of module import configuration
	if utils.Implements[*parse.ImportStatement](ancestors[ancestorCount-3]) {
		configObject := objectLiteral

		for _, sectionName := range inoxconsts.IMPORT_CONFIG_SECTION_NAMES {
			if configObject.HasNamedProp(sectionName) {
				//ignore properties that are already present.
				continue
			}

			if !hasPrefixCaseInsensitive(sectionName, ident.Name) {
				//ignore properties that don't have ident.Name as prefix.
				continue
			}

			suffix := ""
			if prop.HasNoKey() {
				suffix = ": "

				valueCompletion, ok := MODULE_IMPORT_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
				if ok {
					suffix += valueCompletion
				}
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				LabelDetail:           MODULE_IMPORT_SECTION_LABEL_DETAILS[sectionName],
				MarkdownDocumentation: MODULE_IMPORT_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
			})
		}
		return completions
	}

	//suggest sections of lthread meta
	if ancestorCount > 3 && utils.Implements[*parse.SpawnExpression](ancestors[ancestorCount-3]) &&
		objectLiteral == ancestors[ancestorCount-3].(*parse.SpawnExpression).Meta {
		for _, sectionName := range symbolic.LTHREAD_SECTION_NAMES {
			if hasPrefixCaseInsensitive(sectionName, ident.Name) {

				suffix := ""
				if prop.HasNoKey() {
					suffix = ": "

					valueCompletion, ok := LTHREAD_META_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
					if ok {
						suffix += valueCompletion
					}
				}

				completions = append(completions, Completion{
					ShownString:           sectionName + suffix,
					Value:                 sectionName + suffix,
					LabelDetail:           LTHREAD_META_SECTION_LABEL_DETAILS[sectionName],
					MarkdownDocumentation: LTHREAD_META_SECTION_DOC[sectionName],
					Kind:                  defines.CompletionItemKindVariable,
				})
			}
		}
		return completions
	}

	//case: the current property is a property of the permissions section of the manifest.
	if ancestorCount >= 6 && utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-2]) &&
		utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-3]) &&
		ancestors[ancestorCount-3].(*parse.ObjectProperty).HasNameEqualTo(inoxconsts.MANIFEST_PERMS_SECTION_NAME) &&
		utils.Implements[*parse.Manifest](ancestors[ancestorCount-5]) {

		for _, info := range permbase.PERMISSION_KINDS {
			if !hasPrefixCaseInsensitive(info.Name, ident.Name) {
				continue
			}

			detail := MAJOR_PERM_KIND_TEXT

			if info.PermissionKind.IsMinor() {
				detail = MINOR_PERM_KIND_TEXT
			}

			completions = append(completions, Completion{
				ShownString: info.Name,
				Value:       info.Name,
				Kind:        defines.CompletionItemKindVariable,
				LabelDetail: detail,
			})
		}

		return completions
	}

	//case: the current property is in the 'allow' object in a module import statement.
	if ancestorCount >= 6 && utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-2]) &&
		utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-3]) &&
		ancestors[ancestorCount-3].(*parse.ObjectProperty).HasNameEqualTo(inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME) &&
		utils.Implements[*parse.ImportStatement](ancestors[ancestorCount-5]) {

		for _, info := range permbase.PERMISSION_KINDS {
			if !hasPrefixCaseInsensitive(info.Name, ident.Name) {
				continue
			}

			detail := MAJOR_PERM_KIND_TEXT

			if info.PermissionKind.IsMinor() {
				detail = MINOR_PERM_KIND_TEXT
			}

			completions = append(completions, Completion{
				ShownString: info.Name,
				Value:       info.Name,
				Kind:        defines.CompletionItemKindVariable,
				LabelDetail: detail,
			})
		}

		return completions
	}

	properties, ok := search.state.Global.SymbolicData.GetAllowedNonPresentProperties(objectLiteral)
	if ok {
		for _, name := range properties {
			if hasPrefixCaseInsensitive(name, ident.Name) {
				propNameAndColon := quotePropNameIfNecessary(name) + ": "
				completions = append(completions, Completion{
					ShownString: propNameAndColon,
					Value:       propNameAndColon,
					Kind:        defines.CompletionItemKindProperty,
				})
			}
		}
	}

	if strings.HasPrefix(ident.Name, "HX") {
		completions = append(completions, getHTMXResponseHeaderNames(ident, search)...)
	}

	return
}

func quotePropNameIfNecessary(name string) string {
	if parse.IsValidIdent(name) {
		return name
	}
	quoted := utils.Must(json.Marshal(name))
	return utils.BytesAsString(quoted)
}
