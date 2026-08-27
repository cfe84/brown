package checks

// BandwidthRegistry returns a registry containing only a bandwidth check.
func BandwidthRegistry(baseURL string) *Registry {
	r := NewRegistry()
	r.Register(&BandwidthCheck{BaseURL: baseURL})
	return r
}
