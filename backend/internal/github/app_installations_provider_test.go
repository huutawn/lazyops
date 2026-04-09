package github

import "testing"

func TestHasGitHubNextPage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{
			name:   "empty header",
			header: "",
			want:   false,
		},
		{
			name:   "single last page",
			header: `<https://api.github.com/user/installations?page=2>; rel="last"`,
			want:   false,
		},
		{
			name:   "contains next rel",
			header: `<https://api.github.com/user/installations?page=2>; rel="next", <https://api.github.com/user/installations?page=9>; rel="last"`,
			want:   true,
		},
		{
			name:   "contains prev and first only",
			header: `<https://api.github.com/user/installations?page=1>; rel="first", <https://api.github.com/user/installations?page=3>; rel="prev"`,
			want:   false,
		},
	}

	for _, test := range tests {
		if got := hasGitHubNextPage(test.header); got != test.want {
			t.Fatalf("%s: expected %v, got %v", test.name, test.want, got)
		}
	}
}
