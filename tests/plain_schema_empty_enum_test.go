package gateway

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Google refuses an empty enum value in a function declaration. plainSchema
// drops the whole enum keyword, including one written by const, so an empty
// choice cannot be sent or written back by the model. A valid enum and the
// existing rule against non-string values stay as they were.
func TestPlainSchemaDropsEnumWithEmptyValue(t *testing.T) {
	in := `{"type":"object","properties":{
		"emptyEnum":{"type":"string","enum":["","minimal","low","medium","high"]},
		"emptyConst":{"type":"string","const":""},
		"keptEnum":{"type":"string","enum":["a","b"]},
		"mixedEnum":{"type":"string","enum":["a",1]}
	}}`
	want := `{"type":"object","properties":{
		"emptyEnum":{"type":"string"},
		"emptyConst":{"type":"string"},
		"keptEnum":{"type":"string","enum":["a","b"]},
		"mixedEnum":{"type":"string"}
	}}`
	var got, exp any
	if err := json.Unmarshal(plainSchema(json.RawMessage(in)), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &exp); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, exp) {
		b, _ := json.Marshal(got)
		t.Errorf("got %s", b)
	}
}
