package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/tutorial"
)

func TestSelectLessons(t *testing.T) {
	all := tutorial.Lessons()

	got, err := selectLessons(nil)
	if err != nil || len(got) != len(all) {
		t.Errorf("selectLessons(nil) = %d lessons, %v; want all (%d)", len(got), err, len(all))
	}

	got, err = selectLessons([]string{"2"})
	if err != nil || len(got) != 1 || got[0].Num != 2 {
		t.Errorf("selectLessons([2]) = %v, %v; want lesson 2", got, err)
	}

	for _, bad := range [][]string{{"0"}, {"999"}, {"abc"}} {
		if _, err := selectLessons(bad); err == nil {
			t.Errorf("selectLessons(%v) expected an error", bad)
		}
	}
}

func TestListLessons(t *testing.T) {
	var buf bytes.Buffer
	listLessons(&buf)
	out := buf.String()
	for _, l := range tutorial.Lessons() {
		if !strings.Contains(out, l.Title) {
			t.Errorf("listLessons missing %q", l.Title)
		}
	}
}
