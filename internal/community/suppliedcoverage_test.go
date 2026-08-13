package community

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// ValidateGrounding checks a reply against Supplied() alone. A carrier the
// prompt gains and Supplied() does not is content grounding stops seeing.

// The failure is silent, which is what makes it worth a test. Nothing errors.
// The validator's coverage shrinks while every existing check still passes.
func TestSuppliedCoversEveryPromptCarrier(t *testing.T) {
	t.Parallel()
	value := reflect.ValueOf(&TurnPrompt{}).Elem()
	fields := value.Type()

	// A sentinel per field, so a field Supplied() drops is named rather than
	// inferred from a diff of the whole prompt.
	sentinels := make(map[string]string, fields.NumField())
	for i := range fields.NumField() {
		field := fields.Field(i)
		if field.Type.Kind() != reflect.String {
			t.Fatalf("TurnPrompt.%s is a %s, which this test cannot fill. Extend it "+
				"and make sure Supplied() carries the field", field.Name, field.Type.Kind())
		}
		sentinel := fmt.Sprintf("carrier-sentinel-%s", field.Name)
		sentinels[field.Name] = sentinel
		value.Field(i).SetString(sentinel)
	}
	if len(sentinels) == 0 {
		t.Fatal("TurnPrompt declares no fields, so this test checks nothing")
	}

	supplied := value.Interface().(TurnPrompt).Supplied()
	for name, sentinel := range sentinels {
		if !strings.Contains(supplied, sentinel) {
			t.Errorf("TurnPrompt.%s reaches the model and Supplied() omits it, so "+
				"ValidateGrounding cannot see it. Add the field to Supplied()", name)
		}
	}
}

// A carrier that is empty must contribute nothing, or an absent section becomes
// grounding for a reply that invented it.
func TestSuppliedOmitsAnEmptyCarrier(t *testing.T) {
	t.Parallel()
	prompt := TurnPrompt{Message: "only a message"}
	if prompt.Supplied() != "only a message" {
		t.Errorf("Supplied() = %q for a message-only prompt", prompt.Supplied())
	}
	empty := TurnPrompt{}
	if empty.Supplied() != "" {
		t.Errorf("an empty prompt supplied %q", empty.Supplied())
	}
}

// The sentinel test passes trivially if Supplied() ever returns everything by
// concatenating the struct, so this pins that it is still a selective render.
func TestSuppliedIsNotJustTheWholeStruct(t *testing.T) {
	t.Parallel()
	prompt := TurnPrompt{System: "sys", Context: "ctx", Message: "msg"}
	supplied := prompt.Supplied()
	for _, forbidden := range []string{"TurnPrompt", "System:", "Context:", "Message:"} {
		if strings.Contains(supplied, forbidden) {
			t.Errorf("Supplied() carries the field name %q, so it is rendering the "+
				"struct rather than its contents", forbidden)
		}
	}
}
