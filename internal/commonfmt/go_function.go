package commonfmt

import (
	"errors"
	"reflect"
	"strings"
)

func FormatGoFunctionSignature(f any) string {
	t := reflect.TypeOf(f)
	if t.Kind() != reflect.Func {
		panic(errors.New("argument is not a function"))
	}

	buf := strings.Builder{}
	buf.WriteString("nativefn(")
	for i := 0; i < t.NumIn(); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(t.In(i).String())
	}
	buf.WriteString(")")

	if numOut := t.NumOut(); numOut > 0 {
		if numOut > 1 {
			buf.WriteString(" (")
		} else {
			buf.WriteString(" ")
		}
		for i := 0; i < t.NumOut(); i++ {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(t.Out(i).String())
		}
		if numOut > 1 {
			buf.WriteString(")")
		}
	}

	return buf.String()
}
