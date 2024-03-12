package lsp

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
	"testing"

	defines "github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func TestMethodsGen(t *testing.T) {
	t.SkipNow()

	//todo: fix
	res := generate(methods)

	err := ioutil.WriteFile("./methods_gen.go", []byte(res), 0777)
	if err != nil {
		panic(err)
	}
}

type typ_ struct {
	typName string
	typ     string
}

func removeNamePrefix(s string) string {
	n := strings.Split(s, ".")
	name := n[len(n)-1]
	return name
}

func getTypeOne(i interface{}) typ_ {

	if _, ok := i.(_any); ok {
		return typ_{"any", "any"}
	}

	t := reflect.TypeOf(i)
	strT := t.String()
	name := removeNamePrefix(strT)
	if t.Kind() == reflect.Slice {
		return typ_{typ: strT, typName: "Slice" + removeNamePrefix(t.Elem().Name())}
	}
	return typ_{name, strT}
}

func getType(i interface{}) []typ_ {
	res := []typ_{}
	if isNil(i) {
		return res
	}
	t := reflect.TypeOf(i)
	if t == reflect.TypeOf(or{}) {
		orItems := i.(or)
		for _, item := range orItems {
			res = append(res, getTypeOne(item))
		}
		return res
	}
	res = append(res, getTypeOne(i))
	return res
}

var test = method{
	Name:        "Initialize",
	Args:        defines.InitializeParams{},
	Result:      or{defines.InitializeResult{}, defines.DocumentLink{}},
	Error:       defines.InitializeError{},
	Code:        defines.InitializeErrorUnknownProtocolVersion,
	WithBuiltin: true,
}

func firstLow(s string) string {
	if len(s) < 1 {
		return s
	}
	return strings.ToLower(s[0:1]) + s[1:]
}

func firstUp(s string) string {
	if len(s) < 1 {
		return s
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}

func generateOne(name, regName, args, result, error, code string, withBuiltin bool, rateLimits string, sensitiveData bool) (string, string, string) {
	name = firstUp(name)
	nameFirstLow := firstLow(name)
	structFieldFmt := structItemTemp
	methodsTempFmt := methodsTemp
	if result == "any" {
		structFieldFmt = interfRespStructItemTemp
		methodsTempFmt = interfRespMethodsTemp
	}

	structField := fmt.Sprintf(structFieldFmt, name, args, result, error)
	method := fmt.Sprintf(methodsTempFmt, name, args, result, error, name)

	defaultOpt := noBuiltinTemp
	if withBuiltin {
		defaultOpt = fmt.Sprintf(builtinTemp, name, code)
	}
	rpcHandler := fmt.Sprintf(jsonrpcHandlerTemp, nameFirstLow, args, name, name, code, defaultOpt)
	retArgs := "&" + args + "{}"
	if args == "interface{}" {
		retArgs = "nil"
	}
	defaultRet := ""
	if !withBuiltin {
		defaultRet = fmt.Sprintf(methodInfoDefaultTemp, name)
	}
	methodsInfo := fmt.Sprintf(methodsInfoTemp, nameFirstLow, defaultRet, regName, retArgs, nameFirstLow, rateLimits, sensitiveData)
	getInfo := fmt.Sprintf(getInfoItemTemp, nameFirstLow)
	return structField, fmt.Sprintf("%s\n%s\n%s", method, rpcHandler, methodsInfo), getInfo
}

func generateOneNoResp(name, regName, args, error, code string, withBuiltin bool, rateLimits string, sensitiveData bool) (string, string, string) {
	name = firstUp(name)
	nameFirstLow := firstLow(name)
	structField := fmt.Sprintf(noRespStructItemTemp, name, args, error)
	method := fmt.Sprintf(noRespMethodsTemp, name, args, error, name)
	defaultOpt := noBuiltinTemp
	if withBuiltin {
		defaultOpt = fmt.Sprintf(noRespBuiltinTemp, name, code)
	}
	rpcHandler := fmt.Sprintf(noRespJsonrpcHandlerTemp, nameFirstLow, args, name, name, code, defaultOpt)
	retArgs := "&" + args + "{}"
	if args == "interface{}" {
		retArgs = "nil"
	}
	defaultRet := ""
	if !withBuiltin {
		defaultRet = fmt.Sprintf(methodInfoDefaultTemp, name)
	}
	methodsInfo := fmt.Sprintf(methodsInfoTemp, nameFirstLow, defaultRet, regName, retArgs, nameFirstLow, rateLimits, sensitiveData)
	getInfo := fmt.Sprintf(getInfoItemTemp, nameFirstLow)
	return structField, fmt.Sprintf("%s\n%s\n%s", method, rpcHandler, methodsInfo), getInfo
}

func generate(items []method) string {
	codeBlock1 := []string{}
	codeBlock2 := []string{}
	codeBlock3 := []string{}
	for _, item := range items {
		name := item.Name
		regName := item.RegisterName
		if regName == "" {
			regName = firstLow(name)
		}
		builtin := item.WithBuiltin
		args := getType(item.Args)
		result := getType(item.Result)
		error := getType(item.Error)
		code := fmt.Sprintf("%d", item.Code)
		rateLimits := strings.Join(utils.MapSlice(item.RateLimits, strconv.Itoa), ", ")
		hasSensitiveData := item.SensitiveData

		if isNil(item.Code) {
			code = "0"
		}
		errorT := ""
		if len(error) == 0 {
			errorT = "error"
		} else if len(error) == 1 {
			errorT = "*" + error[0].typ
		} else {
			panic(fmt.Sprintf("not supported %v", item))
		}
		if len(args) == 0 {
			args = append(args, typ_{typ: "interface{}", typName: "nil"})
		}
		if len(result) == 0 {
			if len(args) == 0 {
				panic(fmt.Sprintf("not supported %v", item))
			}
			if len(args) == 1 {
				a, b, c := generateOneNoResp(name, regName, args[0].typ, errorT, code, builtin, rateLimits, hasSensitiveData)
				codeBlock1 = append(codeBlock1, a)
				codeBlock2 = append(codeBlock2, b)
				codeBlock3 = append(codeBlock3, c)
			} else {
				for _, n := range args {
					newName := name + "With" + n.typName
					a, b, c := generateOneNoResp(newName, regName, n.typ, errorT, code, builtin, rateLimits, hasSensitiveData)
					codeBlock1 = append(codeBlock1, a)
					codeBlock2 = append(codeBlock2, b)
					codeBlock3 = append(codeBlock3, c)
				}
			}
		}

		if len(result) == 1 {
			if len(args) == 0 {
				panic(fmt.Sprintf("not supported %v", item))
			}
			if len(args) == 1 {
				a, b, c := generateOne(name, regName, args[0].typ, result[0].typ, errorT, code, builtin, rateLimits, hasSensitiveData)
				codeBlock1 = append(codeBlock1, a)
				codeBlock2 = append(codeBlock2, b)
				codeBlock3 = append(codeBlock3, c)
			} else {
				for _, n := range args {
					newName := name + "With" + n.typName
					a, b, c := generateOne(newName, regName, n.typ, result[0].typ, errorT, code, builtin, rateLimits, hasSensitiveData)
					codeBlock1 = append(codeBlock1, a)
					codeBlock2 = append(codeBlock2, b)
					codeBlock3 = append(codeBlock3, c)
				}
			}
		}

		if len(result) > 1 {
			if len(args) == 0 {
				panic(fmt.Sprintf("not supported %v", item))
			}
			if len(args) == 1 {
				for _, n := range result {
					newName := name + "With" + n.typName
					a, b, c := generateOne(newName, regName, args[0].typ, n.typ, errorT, code, builtin, rateLimits, hasSensitiveData)
					codeBlock1 = append(codeBlock1, a)
					codeBlock2 = append(codeBlock2, b)
					codeBlock3 = append(codeBlock3, c)
				}
			} else {
				for _, r := range result {
					for _, n := range args {
						newName := name + "With" + n.typName + "With" + r.typName
						a, b, c := generateOne(newName, regName, r.typ, n.typ, errorT, code, builtin, rateLimits, hasSensitiveData)
						codeBlock1 = append(codeBlock1, a)
						codeBlock2 = append(codeBlock2, b)
						codeBlock3 = append(codeBlock3, c)
					}
				}

			}
		}
	}
	pkg := "// code gen by methods_gen_test.go, do not edit!\npackage lsp\n"

	pkg += `
	import (
		"context"
	
		"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
		"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	)
	`

	code1 := strings.Join(codeBlock1, "\n")
	code2 := strings.Join(codeBlock2, "\n")
	code3 := strings.Join(codeBlock3, "\n")
	code1 = fmt.Sprintf(structTemp, code1)
	code3 = fmt.Sprintf(getInfoTemp, code3)
	return pkg + code1 + "\n" + code2 + "\n" + code3
}
