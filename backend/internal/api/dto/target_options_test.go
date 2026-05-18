package dto

import (
	"reflect"
	"strings"
	"testing"
)

func requireUseAmassField(t *testing.T, sample any, wantPointer bool) {
	t.Helper()

	typ := reflect.TypeOf(sample)
	field, ok := typ.FieldByName("UseAmass")
	if !ok {
		t.Fatalf("%s is missing UseAmass field", typ.Name())
	}

	if got := strings.Split(field.Tag.Get("json"), ",")[0]; got != "use_amass" {
		t.Fatalf("%s.UseAmass json tag = %q, want use_amass", typ.Name(), field.Tag.Get("json"))
	}

	if wantPointer {
		if field.Type.Kind() != reflect.Pointer || field.Type.Elem().Kind() != reflect.Bool {
			t.Fatalf("%s.UseAmass must be *bool, got %s", typ.Name(), field.Type)
		}
		return
	}

	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("%s.UseAmass must be bool, got %s", typ.Name(), field.Type)
	}
}

func TestTargetDTOsExposeUseAmass(t *testing.T) {
	requireUseAmassField(t, CreateTargetRequest{}, true)
	requireUseAmassField(t, UpdateTargetRequest{}, true)
	requireUseAmassField(t, TargetExportItem{}, false)
	requireUseAmassField(t, TargetResponse{}, false)
}
