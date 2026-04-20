package filters

// Registry holds an ordered list of filters. Order matters: the first
// filter whose Match returns true wins. Register more specific filters
// before more general ones.
type Registry struct {
	filters []Filter
}

// New returns an empty registry. Callers wire up concrete filters via
// Register.
func New() *Registry {
	return &Registry{}
}

// Register appends f to the registry. Not safe for concurrent use —
// wiring is expected to happen once at startup.
func (r *Registry) Register(f Filter) {
	r.filters = append(r.filters, f)
}

// Lookup returns the first filter that matches the invocation, or nil if
// none do. A nil return means "pass-through" — callers must emit the raw
// output unchanged.
func (r *Registry) Lookup(cmd string, args []string) Filter {
	for _, f := range r.filters {
		if f.Match(cmd, args) {
			return f
		}
	}
	return nil
}

// Default returns a registry with all built-in filters registered in the
// recommended order. Kept here so cmd/wafi does not need to know every
// concrete filter type.
func Default() *Registry {
	r := New()
	r.Register(GitStatus{})
	r.Register(GitDiff{})
	r.Register(GitLog{})
	return r
}
