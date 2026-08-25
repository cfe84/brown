package checks

// Registry holds an ordered list of checks to run.
type Registry struct {
	checks []Check
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a check to the registry.
func (r *Registry) Register(c Check) {
	r.checks = append(r.checks, c)
}

// All returns every registered check.
func (r *Registry) All() []Check {
	return r.checks
}

// DefaultRegistry returns the standard set of diagnostic checks.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&InterfaceCheck{})
	r.Register(&GatewayCheck{})
	r.Register(&DNSCheck{})
	r.Register(&PingCheck{})
	r.Register(&WiFiCheck{})
	r.Register(&ConnectivityCheck{})
	return r
}

// ConnectivityRegistry returns only the gateway-versus-internet diagnostic.
func ConnectivityRegistry() *Registry {
	r := NewRegistry()
	r.Register(&ConnectivityCheck{})
	return r
}
