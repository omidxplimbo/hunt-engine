package worker

import "testing"

func TestNormalizeSubdomain(t *testing.T) {
	root := "example.com"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trims_and_lowercases",
			in:   "  API.Example.com  ",
			want: "api.example.com",
		},
		{
			name: "removes_www_prefix_only",
			in:   "WWW.Api.Example.com",
			want: "api.example.com",
		},
		{
			name: "rejects_root_domain",
			in:   "example.com",
			want: "",
		},
		{
			name: "rejects_www_root_domain",
			in:   "www.example.com",
			want: "",
		},
		{
			name: "rejects_non_subdomain_suffix_trick",
			in:   "foo.notexample.com",
			want: "",
		},
		{
			name: "rejects_empty",
			in:   "   ",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeSubdomain(tc.in, root); got != tc.want {
				t.Fatalf("normalizeSubdomain(%q, %q) = %q; want %q", tc.in, root, got, tc.want)
			}
		})
	}
}

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

	// We only assert a few key expectations; full normalization happens later via normalizeSubdomain.
	wantSet := map[string]bool{
		"api.example.com":     false,
		"www.example.com":     false,
		"dev.example.com":     false,
		"shop.example.com":    false,
		"foo.notexample.com":  false, // parseCrtshJSON only filters by suffix; boundary is enforced in normalizeSubdomain
		"example.com":         false, // allowed here; normalizeSubdomain will drop root
	}

	for _, s := range got {
		if _, ok := wantSet[s]; ok {
			wantSet[s] = true
		}
	}

	// Ensure we got at least the expected core entries from CN + name_value parsing.
	for k, seen := range wantSet {
		// skip the known edge cases that depend on later normalization rules
		if k == "foo.notexample.com" || k == "example.com" {
			continue
		}
		if !seen {
			t.Fatalf("expected %q to be present in parseCrtshJSON results; got=%v", k, got)
		}
	}
}


