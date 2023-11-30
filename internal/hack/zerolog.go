package hack

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

func ReplaceLoggerStringField(logger zerolog.Logger, key string, newValue string) {
	field := reflect.ValueOf(&logger).Elem().FieldByName("context")
	context := getUnexportedField(field).([]byte)

	i := 1
	for i < len(context) {
		b := context[i]

		if b == '"' &&
			//make sure we found a key
			context[i-1] != ':' &&
			//check that the found key starts with <key>
			i < len(context)-len(key)-3 && bytes.HasPrefix(context[i+1:], utils.StringAsBytes(key)) {

			i += ( /*move to start of key*/ 1 + /*move to closing quote*/ len(key))

			if context[i] != '"' {
				//the found key has no the same name
				i++
				continue
			}

			i += ( /*move to colon*/ 1 + /*move to opening quote of value*/ 1)

			if context[i] != '"' {
				panic(fmt.Errorf("field %q has not a string value", key))
			}

			oldValueStart := i //quote included
			oldValueEnd := -1  //exclusive

			//find the end index of the old value

			ind := oldValueStart
			for ind < len(context) {
				b := context[ind]
				if b != '"' {
					continue
				}
				prevBackslashes := utils.CountPrevBackslashes(context, int32(ind))
				if prevBackslashes%2 == 0 {
					oldValueEnd = ind + 1
					break
				}
			}

			//replace the old value with the new one
			if oldValueEnd <= 0 {
				panic(fmt.Errorf("the current value of the field %q is an unterminated string", key))
			}

			var newContext []byte
			newContext = append(newContext, context[:oldValueStart]...)
			newContext = append(newContext, newValue...)

			if oldValueEnd < len(context) {
				newContext = append(newContext, context[oldValueEnd:]...)
			}

			setUnexportedField(field, newContext)
			return
		}

		i++
	}

}
