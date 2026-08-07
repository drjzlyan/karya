package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/skills"
)

func TestRegistriesDefaultPlusAdded(t *testing.T) {
	p := config.Paths{Config: t.TempDir()}
	// Only the default before adding.
	if regs := registries(p); len(regs) != 1 || regs[0] != defaultRegistry {
		t.Fatalf("default registries = %v", regs)
	}
	if err := addRegistry(p, "https://example.com/reg"); err != nil {
		t.Fatal(err)
	}
	if err := addRegistry(p, "  https://example.com/reg2  "); err != nil {
		t.Fatal(err)
	}
	regs := registries(p)
	if len(regs) != 3 || regs[1] != "https://example.com/reg" || regs[2] != "https://example.com/reg2" {
		t.Fatalf("registries after add = %v", regs)
	}
	// Persisted to the registries file.
	if _, err := os.Stat(filepath.Join(p.Config, "registries")); err != nil {
		t.Fatalf("registries file not written: %v", err)
	}
}

func TestSourceForKind(t *testing.T) {
	if _, ok := sourceFor("https://x/y").(*skills.HTTPSource); !ok {
		t.Fatal("https ref should be an HTTP source")
	}
	if _, ok := sourceFor("/local/dir").(skills.DirSource); !ok {
		t.Fatal("local ref should be a dir source")
	}
}
