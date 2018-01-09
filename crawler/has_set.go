package crawler

import "sync"

// HasSet implements a concurrent-safe set implementation for quick checksups of
// string values. We implement this to leverage the fact that using `struct{}` as
// value does not allocate memory.
type HasSet struct {
	ml   sync.RWMutex
	data map[string]struct{}
}

// NewHasSet returns a new instance of a hasset.
func NewHasSet() *HasSet {
	return &HasSet{
		data: map[string]struct{}{},
	}
}

// Has returns true/false if giving string value exists in set.
func (h *HasSet) Has(val string) bool {
	h.ml.RLock()
	defer h.ml.RUnlock()

	_, found := h.data[val]
	return found
}

// Add adds all provided items into sets.
func (h *HasSet) Add(items ...string) {
	if len(items) == 0 {
		return
	}

	h.ml.Lock()
	defer h.ml.Unlock()

	for _, item := range items {
		h.data[item] = struct{}{}
	}
}
