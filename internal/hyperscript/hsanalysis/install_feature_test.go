package hsanalysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyResolvedInstalls(t *testing.T) {
	t.Run("base case", func(t *testing.T) {
		resolvedBehavior := &Behavior{
			Name:                          "A",
			FullName:                      "A",
			InitialElementScopeVarNames:   []string{":a"},
			InitializedDataAttributeNames: []string{"data-x"},
			HandledEvents:                 []DOMEvent{{Type: "X"}},
		}

		installs := []*InstallFeature{{BehaviorFullName: "A", ResolvedBehavior: resolvedBehavior}}

		var elementScopedVarNames []string
		var dataAttributeNames []string
		var handledEvents []DOMEvent

		var appliedInstalls []*InstallFeature

		errs := applyResolvedInstalls(installs, &appliedInstalls, &elementScopedVarNames, &dataAttributeNames, &handledEvents)

		assert.Empty(t, errs)
		if !assert.Equal(t, []*InstallFeature{installs[0]}, appliedInstalls) {
			return
		}
		assert.Equal(t, []string{":a"}, elementScopedVarNames)
		assert.Equal(t, []string{"data-x"}, dataAttributeNames)
		assert.Equal(t, []DOMEvent{{Type: "X"}}, handledEvents)

		//Calling the function again should have no effect since the installs are applied.

		errs = applyResolvedInstalls(installs, &appliedInstalls, &elementScopedVarNames, &dataAttributeNames, &handledEvents)
		assert.Empty(t, errs)
		assert.Equal(t, []string{":a"}, elementScopedVarNames)
		assert.Equal(t, []string{"data-x"}, dataAttributeNames)
		assert.Equal(t, []DOMEvent{{Type: "X"}}, handledEvents)
	})

	t.Run("if some variables or data attributes or event types are common, no duplicate should appear", func(t *testing.T) {
		resolvedBehavior := &Behavior{
			Name:                          "A",
			FullName:                      "A",
			InitialElementScopeVarNames:   []string{":a", ":b"},
			InitializedDataAttributeNames: []string{"data-x", "data-y"},
			HandledEvents:                 []DOMEvent{{Type: "X"}, {Type: "Y"}},
		}

		installs := []*InstallFeature{{BehaviorFullName: "A", ResolvedBehavior: resolvedBehavior}}

		elementScopedVarNames := []string{":a"}
		dataAttributeNames := []string{"data-x"}
		handledEvents := []DOMEvent{{Type: "X"}}

		var appliedInstalls []*InstallFeature

		errs := applyResolvedInstalls(installs, &appliedInstalls, &elementScopedVarNames, &dataAttributeNames, &handledEvents)

		assert.Empty(t, errs)
		if !assert.Equal(t, []*InstallFeature{installs[0]}, appliedInstalls) {
			return
		}
		assert.Equal(t, []string{":a", ":b"}, elementScopedVarNames)
		assert.Equal(t, []string{"data-x", "data-y"}, dataAttributeNames)
		assert.Equal(t, []DOMEvent{{Type: "X"}, {Type: "Y"}}, handledEvents)

		//Calling the function again should have no effect since the installs are applied.

		errs = applyResolvedInstalls(installs, &appliedInstalls, &elementScopedVarNames, &dataAttributeNames, &handledEvents)
		assert.Empty(t, errs)
		assert.Equal(t, []string{":a", ":b"}, elementScopedVarNames)
		assert.Equal(t, []string{"data-x", "data-y"}, dataAttributeNames)
		assert.Equal(t, []DOMEvent{{Type: "X"}, {Type: "Y"}}, handledEvents)
	})

	t.Run("passing an unresolved install should have no effect", func(t *testing.T) {

		installs := []*InstallFeature{{BehaviorFullName: "A", ResolvedBehavior: nil}}

		elementScopedVarNames := []string{":a"}
		dataAttributeNames := []string{"data-x"}
		handledEvents := []DOMEvent{{Type: "X"}}

		var appliedInstalls []*InstallFeature

		errs := applyResolvedInstalls(installs, &appliedInstalls, &elementScopedVarNames, &dataAttributeNames, &handledEvents)

		assert.Empty(t, errs)
		if !assert.Empty(t, appliedInstalls) {
			return
		}
		assert.Empty(t, errs)
		assert.Equal(t, []string{":a"}, elementScopedVarNames)
		assert.Equal(t, []string{"data-x"}, dataAttributeNames)
		assert.Equal(t, []DOMEvent{{Type: "X"}}, handledEvents)
	})
}
