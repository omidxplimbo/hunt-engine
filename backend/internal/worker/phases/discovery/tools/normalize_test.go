package tools

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
			in:   " API.Example.com ",
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
			in:   " ",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSubdomain(tc.in, root); got != tc.want {
				t.Fatalf("NormalizeSubdomain(%q, %q) = %q; want %q", tc.in, root, got, tc.want)
			}
		})
	}
}
