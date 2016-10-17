package views

import (
	"errors"
	"fmt"
	"reflect"
	"unicode"

	"github.com/saintpete/logrole/config"
)

type RedactedMessage struct {
	mv *Message
}

func NewRedactedMessage(mv *Message) *RedactedMessage {
	return &RedactedMessage{mv: mv}
}

const redacted = "[redacted]"

// Call calls the given method name on a Message. If the result is
// a PermissionDenied error, return the string "[redacted]" and no error.
//
// This should be used by templates to redact methods. It would be nicer to
// just call the Message directly, but returning an error from a niladic method
// immediately halts template execution, so we need this wrapper around the
// function behavior.
func (r *RedactedMessage) Call(mname string) (interface{}, error) {
	if mname == "" {
		return nil, errors.New("Call() with empty string")
	}
	for _, char := range mname {
		if !unicode.IsUpper(char) {
			return nil, errors.New("Cannot call private method")
		}
		// only check first character
		break
	}
	t := reflect.ValueOf(r.mv)
	m := t.MethodByName(mname)
	if !m.IsValid() {
		return nil, fmt.Errorf("Invalid method: %s", mname)
	}
	vals := m.Call([]reflect.Value{})
	if len(vals) != 2 {
		return nil, fmt.Errorf("Expected to get two values back, got %d", len(vals))
	}
	if vals[1].IsNil() {
		return vals[0].Interface(), nil
	}
	if reflect.DeepEqual(vals[1].Interface(), config.PermissionDenied) {
		return redacted, nil
	}
	return nil, vals[1].Interface().(error)
}
