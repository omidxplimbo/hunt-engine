package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestTargetHasUseAmassOption(t *testing.T) {
	field, ok := reflect.TypeOf(Target{}).FieldByName("UseAmass")
	if !ok {
		t.Fatalf("Target is missing UseAmass field")
	}

	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("Target.UseAmass must be bool, got %s", field.Type)
	}

	if got := field.Tag.Get("json"); strings.Split(got, ",")[0] != "use_amass" {
		t.Fatalf("Target.UseAmass json tag = %q, want use_amass", got)
	}

	gormTag := field.Tag.Get("gorm")
	if !strings.Contains(gormTag, "default:false") {
		t.Fatalf("Target.UseAmass gorm tag = %q, want default:false", gormTag)
	}
}
