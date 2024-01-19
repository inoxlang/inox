package core

import (
	"fmt"
	"io"
	"reflect"
)

func optimizeBytecode(b *Bytecode, tracer io.Writer) {
	deduplicateConstants(b, tracer)
}

func deduplicateConstants(b *Bytecode, tracer io.Writer) {
	constantsMapping := make([]int, len(b.constants))
	ctx := NewContext(ContextConfig{})

	for i := range constantsMapping {
		constantsMapping[i] = -1
	}

	var newConstants []Value

	for i, c1 := range b.constants {
		//if the constant has already been remapped we ignore it
		if constantsMapping[i] >= 0 {
			continue
		}

		newConstantIndex := len(newConstants)
		constantsMapping[i] = newConstantIndex
		newConstants = append(newConstants, c1)
		v := reflect.ValueOf(c1)

		//we ignore values that are not integers, floats, strings nor booleans.
		//TODO: support checked strings
		switch v.Kind() {
		case reflect.Bool, reflect.String:
		default:
			if !v.CanInt() && !v.CanFloat() {
				continue
			}
		}

		for j := i + 1; j < len(b.constants); j++ {
			c2 := b.constants[j]
			if c1.Equal(nil, c2, map[uintptr]uintptr{}, 0) {
				jsonReprConfig := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG}

				if tracer != nil {
					c1Repr := MustGetJSONRepresentationWithConfig(c1.(Serializable), ctx, jsonReprConfig)
					c2Repr := MustGetJSONRepresentationWithConfig(c2.(Serializable), ctx, jsonReprConfig)

					s := fmt.Sprintf("%s (%d) is equal to %s (%d), remapping %d -> %d\n", c1Repr, i, c2Repr, j, j, newConstantIndex)
					tracer.Write([]byte(s))
				}
				constantsMapping[j] = newConstantIndex
			}
		}
	}

	prevConstants := b.constants

	updateConstantReferences := func(fn *CompiledFunction) ([]byte, error) {
		return MapInstructions(
			fn.Instructions,
			prevConstants,
			func(instr []byte, op Opcode, operands, constantIndexOperandIndexes []int, _ []Value, i int) ([]byte, error) {
				for _, operandIndex := range constantIndexOperandIndexes {
					constantIndex := operands[operandIndex]
					newConstantIndex := constantsMapping[constantIndex]
					operands[operandIndex] = newConstantIndex
				}
				return MakeInstruction(op, operands...), nil
			},
		)
	}

	//we update compiled functions' instructions
	for _, c := range b.constants {
		if fn, ok := c.(*InoxFunction); ok && fn.compiledFunction != nil {
			newInstructions, err := updateConstantReferences(fn.compiledFunction)
			if err != nil {
				panic(err)
			}
			fn.compiledFunction.Instructions = newInstructions
		}
	}

	//we update the bytecode's instructions

	newInstructions, err := updateConstantReferences(b.main)
	if err != nil {
		panic(err)
	}

	b.main.Instructions = newInstructions
	b.constants = newConstants
}
