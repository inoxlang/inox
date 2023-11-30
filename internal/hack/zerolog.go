package hack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

// AddReplaceLoggerStringFieldValue replaces the value of the key field in logger if present, otherwise it
// adds the field by doing logger.With().Str(key, newValue).Logger().
func AddReplaceLoggerStringFieldValue(logger zerolog.Logger, key string, newValue string) zerolog.Logger {
	field := reflect.ValueOf(&logger).Elem().FieldByName("context")
	context := getUnexportedField(field).([]byte)

	quotedKey := utils.Must(json.Marshal(key))

	i := 1
	for i < len(context) {
		b := context[i]

		if b == '"' &&
			//make sure we found a key
			context[i-1] != ':' &&
			i < len(context)-len(quotedKey)-2 && bytes.HasPrefix(context[i:], quotedKey) {

			//move to closing quote
			i += len(quotedKey) - 1

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

			ind := oldValueStart + 1
			for ind < len(context) {
				b := context[ind]
				if b != '"' {
					ind++
					continue
				}
				prevBackslashes := utils.CountPrevBackslashes(context, int32(ind))
				if prevBackslashes%2 == 0 {
					oldValueEnd = ind + 1
					break
				}
				ind++
			}

			//replace the old value with the new one
			if oldValueEnd <= 0 {
				panic(fmt.Errorf("the current value of the field %q is an unterminated string", key))
			}

			var newContext []byte
			newContext = append(newContext, context[:oldValueStart]...)
			newContext = append(newContext, '"')
			newContext = append(newContext, newValue...)
			newContext = append(newContext, '"')

			if oldValueEnd < len(context) {
				newContext = append(newContext, context[oldValueEnd:]...)
			}

			setUnexportedField(field, newContext)
			//return passed logger
			return logger
		}

		i++
	}

	//the field was not found so we add it

	return logger.With().Str(key, newValue).Logger()
}
