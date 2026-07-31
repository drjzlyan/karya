package toolreg

import "fmt"

// Registry provides read-only, deterministic access to the tool catalog. It is
// the query layer every higher tier (resolver, installer, health, update) reads
// from, so tool knowledge lives in exactly one place.
type Registry struct {
	all  []Tool
	byID map[string]Tool
}

// New builds the registry from the embedded catalog. It panics on a duplicate
// ID because that is a programming error in the catalog data, caught by tests
// before it can ship.
func New() *Registry {
	r := &Registry{
		all:  make([]Tool, len(registry)),
		byID: make(map[string]Tool, len(registry)),
	}
	copy(r.all, registry)
	for _, t := range r.all {
		if _, dup := r.byID[t.ID]; dup {
			panic("toolreg: duplicate tool id " + t.ID)
		}
		r.byID[t.ID] = t
	}
	return r
}

// Get returns the tool with the given ID and whether it was found.
func (r *Registry) Get(id string) (Tool, bool) {
	t, ok := r.byID[id]
	return t, ok
}

// All returns every tool in catalog order. The slice is a copy; callers may not
// mutate the registry through it.
func (r *Registry) All() []Tool {
	out := make([]Tool, len(r.all))
	copy(out, r.all)
	return out
}

// ByCategory returns the tools in the given category, in catalog order.
func (r *Registry) ByCategory(c Category) []Tool {
	var out []Tool
	for _, t := range r.all {
		if t.Category == c {
			out = append(out, t)
		}
	}
	return out
}

// Plan resolves a set of tool IDs into the full, ordered list to install:
// every requested tool plus its transitive dependencies, with dependencies
// placed before the tools that need them and each tool appearing once. The order
// is deterministic (a depth-first walk in the caller's ID order). It returns an
// error for an unknown ID or a dependency cycle.
func (r *Registry) Plan(ids []string) ([]Tool, error) {
	var out []Tool
	done := make(map[string]bool)    // fully emitted
	onStack := make(map[string]bool) // in the current DFS path (cycle guard)

	var visit func(id string, chain []string) error
	visit = func(id string, chain []string) error {
		if done[id] {
			return nil
		}
		if onStack[id] {
			return fmt.Errorf("dependency cycle: %v", append(chain, id))
		}
		t, ok := r.byID[id]
		if !ok {
			return fmt.Errorf("unknown tool %q", id)
		}
		onStack[id] = true
		for _, dep := range t.Dependencies {
			if err := visit(dep, append(chain, id)); err != nil {
				return err
			}
		}
		onStack[id] = false
		done[id] = true
		out = append(out, t)
		return nil
	}

	for _, id := range ids {
		if err := visit(id, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}
