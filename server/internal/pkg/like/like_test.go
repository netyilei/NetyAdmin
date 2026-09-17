package like

import "testing"

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text unchanged", input: "hello", want: "hello"},
		{name: "percent escaped", input: "50%", want: `50\%`},
		{name: "underscore escaped", input: "a_b", want: `a\_b`},
		{name: "backslash escaped", input: `a\b`, want: `a\\b`},
		{name: "all wildcards", input: "%_%", want: `\%\_\%`},
		{name: "empty", input: "", want: ""},
		{name: "chinese text unchanged", input: "文章标题", want: "文章标题"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeLike(tt.input); got != tt.want {
				t.Errorf("EscapeLike(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLikeContains(t *testing.T) {
	if got := LikeContains("50%off"); got != `%50\%off%` {
		t.Errorf("LikeContains(\"50%%off\") = %q, want %%50\\%%off%%", got)
	}
}
