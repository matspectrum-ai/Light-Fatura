package gateway

type StaticRegistry struct {
	adapters map[string]Adapter
	fallback Adapter
}

func NewRegistry(fallback Adapter, adapters ...Adapter) *StaticRegistry {
	r := &StaticRegistry{adapters: make(map[string]Adapter), fallback: fallback}
	for _, adapter := range adapters {
		if adapter != nil {
			r.adapters[adapter.Name()] = adapter
		}
	}
	return r
}

func (r *StaticRegistry) AdapterFor(record Record) Adapter {
	if adapter := r.adapters[record.Adapter]; adapter != nil {
		return adapter
	}
	if adapter := r.adapters[record.Slug]; adapter != nil {
		return adapter
	}
	return r.fallback
}
