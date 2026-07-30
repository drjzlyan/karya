package cli

import "testing"

func TestParseNewArgs(t *testing.T) {
	cases := []struct {
		name                       string
		args                       []string
		wantLang, wantName, wantPD string
	}{
		{"colon form", []string{"python:myapp"}, "python", "myapp", ""},
		{"colon form trims", []string{" go : mod "}, "go", "mod", ""},
		{"space form", []string{"go", "github.com/u/app"}, "go", "github.com/u/app", ""},
		{"space form with dir", []string{"rust", "app", "/tmp/x"}, "rust", "app", "/tmp/x"},
		{"empty", nil, "", "", ""},
		{"single non-colon", []string{"python"}, "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lang, name, pd := parseNewArgs(c.args)
			if lang != c.wantLang || name != c.wantName || pd != c.wantPD {
				t.Errorf("parseNewArgs(%q) = (%q,%q,%q), want (%q,%q,%q)",
					c.args, lang, name, pd, c.wantLang, c.wantName, c.wantPD)
			}
		})
	}
}
