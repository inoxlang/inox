package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVMStackOverflow(t *testing.T) {
	expectError(t, `
		fn f() { 
			return (1 + f()) 
		}
		f()
	`, nil, ErrStackOverflow)
}

func expectError(t *testing.T, input string, globals map[Identifier]Value, target error) {
	actual, _, e := traceCompile(t, input, nil)

	if !assert.NoError(t, e) {
		return
	}

	vm, err := NewVM(VMConfig{
		Bytecode: actual,
		State: NewGlobalState(NewContext(ContextConfig{
			Permissions: GetDefaultGlobalVarPermissions(),
		})),
	})
	assert.NoError(t, err)

	_, err = vm.Run()
	assert.ErrorIs(t, err, target)
}
