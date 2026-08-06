package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEvidenceNumbers(t *testing.T) {
	dir := t.TempDir()

	p1, err := writeEvidence(dir, "first")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p1) != "VERIFY-1.md" {
		t.Fatalf("first evidence = %s want VERIFY-1.md", filepath.Base(p1))
	}

	p2, _ := writeEvidence(dir, "second")
	if filepath.Base(p2) != "VERIFY-2.md" {
		t.Fatalf("second evidence = %s want VERIFY-2.md", filepath.Base(p2))
	}

	// Unrelated files don't perturb numbering.
	os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte("x"), 0o644)
	p3, _ := writeEvidence(dir, "third")
	if filepath.Base(p3) != "VERIFY-3.md" {
		t.Fatalf("third evidence = %s want VERIFY-3.md", filepath.Base(p3))
	}

	if b, _ := os.ReadFile(p1); string(b) != "first" {
		t.Fatalf("evidence content wrong: %q", b)
	}
}
