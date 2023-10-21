package symbolic

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestJoinValues(t *testing.T) {

	cases := []struct {
		input  []Value
		output Value
	}{
		{[]Value{ANY_INT}, ANY_INT},
		{[]Value{ANY_INT, ANY_INT}, ANY_INT},
		{[]Value{ANY_INT, &String{}}, NewMultivalue(ANY_INT, &String{})},
		{[]Value{&String{}, ANY_INT}, NewMultivalue(&String{}, ANY_INT)},
		{[]Value{&Identifier{name: "foo"}, &Identifier{}}, &Identifier{}},
		{[]Value{&Identifier{}, &Identifier{name: "foo"}}, &Identifier{}},
		{
			[]Value{
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
				NewInexactObject(map[string]Serializable{}, nil, nil),
			},
			NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
				NewInexactObject(map[string]Serializable{}, nil, nil),
			),
		},
		{
			[]Value{
				NewInexactObject(map[string]Serializable{}, nil, nil),
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
			},
			NewMultivalue(
				NewInexactObject(map[string]Serializable{}, nil, nil),
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
			),
		},
		{
			[]Value{
				NewInexactObject(map[string]Serializable{"a": ANY_SERIALIZABLE}, nil, nil),
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
			},
			NewInexactObject(map[string]Serializable{"a": ANY_SERIALIZABLE}, nil, nil),
		},
		{
			[]Value{
				NewList(&String{}),
				NewList(ANY_INT),
			},
			NewMultivalue(
				NewList(&String{}),
				NewList(ANY_INT),
			),
		},
		{
			[]Value{
				NewList(&String{}, &String{}),
				NewList(ANY_INT, &String{}),
			},
			NewMultivalue(
				NewList(&String{}, &String{}),
				NewList(ANY_INT, &String{}),
			),
		},
	}
	for _, testCase := range cases {
		t.Run(t.Name()+"_"+strings.Join(utils.MapSlice(testCase.input, Stringify), " "), func(t *testing.T) {
			output := joinValues(testCase.input)
			assert.Equal(t, testCase.output, output, Stringify(output))
		})
	}
}

func TestWidenToSameStaticTypeInMultivalue(t *testing.T) {

	cases := []struct {
		input  Value
		output Value
	}{
		{ANY_INT, ANY_INT},
		{NewInt(0), NewInt(0)},
		{
			NewMultivalue(NewInt(0), NewInt(1)),
			ANY_INT,
		},
		{
			NewMultivalue(NewInt(0), NewInt(1), NewInt(2)),
			ANY_INT,
		},
		{
			NewMultivalue(NewInt(0), NewInt(1), TRUE),
			NewMultivalue(ANY_INT, TRUE),
		},
		{
			NewMultivalue(NewInt(0), NewInt(1), NewInt(2), TRUE),
			NewMultivalue(ANY_INT, TRUE),
		},
		{
			NewMultivalue(TRUE, NewInt(0), NewInt(1)),
			NewMultivalue(TRUE, ANY_INT),
		},
		{
			NewMultivalue(TRUE, NewInt(0), NewInt(1), NewInt(2)),
			NewMultivalue(TRUE, ANY_INT),
		},
		{
			NewMultivalue(NewInt(0), TRUE, NewInt(1)),
			NewMultivalue(ANY_INT, TRUE),
		},
	}
	for _, testCase := range cases {
		t.Run(t.Name()+"_"+Stringify(testCase.input), func(t *testing.T) {
			output := widenToSameStaticTypeInMultivalue(testCase.input)
			assert.Equal(t, testCase.output, output, Stringify(output))
		})
	}
}
