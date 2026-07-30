package assets

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// docsFS holds the vendored user documentation (docs/*.md) so `karya help`,
// `karya docs` and `karya tutorial` work fully offline from the binary alone.
// The authoritative source is docs/*.md at the repo root; refresh the vendored
// copy with `make sync-docs`. A drift test keeps the two in sync.
//
//go:embed docs/*.md
var docsFS embed.FS

// docsRoot is the embed prefix under which the markdown files live.
const docsRoot = "docs"

// DocTopics returns the available documentation topic names (a doc's file name
// without its .md extension), sorted. E.g. "keymaps", "tutorial".
func DocTopics() []string {
	entries, err := fs.ReadDir(docsFS, docsRoot)
	if err != nil {
		return nil
	}
	topics := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		topics = append(topics, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(topics)
	return topics
}

// Doc returns the markdown content of the named topic (e.g. "tutorial") and
// whether it exists. The topic is the file name without its .md extension.
func Doc(topic string) (string, bool) {
	data, err := docsFS.ReadFile(path.Join(docsRoot, topic+".md"))
	if err != nil {
		return "", false
	}
	return string(data), true
}
