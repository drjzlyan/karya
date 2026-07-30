package assets

import (
	"os"
	"path/filepath"
	"testing"
)

// repoDocsDir is the authoritative docs/ directory at the repo root, relative to
// this package. The embedded copy under internal/assets/docs is vendored from it
// by `make sync-docs`.
const repoDocsDir = "../../docs"

// TestDocsInSyncWithRepo asserts the embedded docs exactly match the source
// docs/*.md at the repo root, so a stale `make sync-docs` fails CI rather than
// shipping outdated help. It is the single-source-of-truth guarantee.
func TestDocsInSyncWithRepo(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join(repoDocsDir, "*.md"))
	if err != nil {
		t.Fatalf("glob repo docs: %v", err)
	}
	if len(sources) == 0 {
		t.Fatalf("no *.md found in %s", repoDocsDir)
	}

	sourceTopics := map[string]bool{}
	for _, src := range sources {
		topic := trimMD(filepath.Base(src))
		sourceTopics[topic] = true

		want, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		got, ok := Doc(topic)
		if !ok {
			t.Errorf("topic %q present in docs/ but not embedded; run `make sync-docs`", topic)
			continue
		}
		if got != string(want) {
			t.Errorf("embedded %q out of sync with docs/%s.md; run `make sync-docs`", topic, topic)
		}
	}

	// And no embedded topic should exist without a source (a deleted doc).
	for _, topic := range DocTopics() {
		if !sourceTopics[topic] {
			t.Errorf("embedded topic %q has no source in docs/; run `make sync-docs`", topic)
		}
	}
}

func TestDocTopics(t *testing.T) {
	topics := DocTopics()
	if len(topics) == 0 {
		t.Fatal("DocTopics returned nothing")
	}
	for _, want := range []string{"tutorial", "keymaps"} {
		if _, ok := Doc(want); !ok {
			t.Errorf("expected embedded topic %q", want)
		}
	}
}

func trimMD(name string) string {
	return name[:len(name)-len(".md")]
}
