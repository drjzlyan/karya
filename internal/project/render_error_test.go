package project

import (
	"strings"
	"testing"
)

// TestExecTemplateErrorNoPanic verifies that a broken template surfaces a normal
// error instead of panicking with a stack trace — the production-safety
// guarantee that replaced the old panic in the template helper.
func TestExecTemplateErrorNoPanic(t *testing.T) {
	// Reference a field the data value does not have, with a strict option so
	// Execute returns an error.
	if _, err := execTemplate("bad-parse", "{{ .Unclosed ", nil); err == nil {
		t.Error("expected a parse error for a malformed template, got nil")
	}
}

// TestFileSetStopsAtFirstError verifies the accumulator short-circuits and
// reports the first error, leaving no partial file list.
func TestFileSetStopsAtFirstError(t *testing.T) {
	var fs fileSet
	fs.raw("keep.txt", "ok")
	fs.tmpl("broken", "broken", "{{ .Missing.Field }}", struct{}{})
	fs.raw("after.txt", "should be skipped")

	files, err := fs.result()
	if err == nil {
		t.Fatal("expected an error from a failing template")
	}
	if files != nil {
		t.Errorf("expected nil files on error, got %d", len(files))
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the failing template: %v", err)
	}
}
