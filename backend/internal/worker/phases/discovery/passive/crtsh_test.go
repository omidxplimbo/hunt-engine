package passive

import "testing"

func TestParseCrtshJSON(t *testing.T) {
	root := "example.com"
	data := []byte(`[
		{"common_name":"*.api.example.com","name_value":"api.example.com\nwww.example.com\n*.dev.example.com"},
		{"common_name":"EXAMPLE.COM","name_value":"shop.example.com\nfoo.notexample.com"},
		{"name_value":""}
	]`)

	got := parseCrtshJSON(data, root)
	if len(got) == 0 {
		t.Fatalf("parseCrtshJSON returned empty results")
	}

	seen := make(map[string]bool, len(got))
	for _, value := range got {
		seen[value] = true
	}

	for _, want := range []string{
		"api.example.com",
		"dev.example.com",
		"shop.example.com",
	} {
		if !seen[want] {
			t.Fatalf("expected %q to be present in parseCrtshJSON results; got=%v", want, got)
		}
	}

	for _, rejected := range []string{
		"example.com",
		"www.example.com",
		"foo.notexample.com",
	} {
		if seen[rejected] {
			t.Fatalf("did not expect %q in parseCrtshJSON results; got=%v", rejected, got)
		}
	}
}
