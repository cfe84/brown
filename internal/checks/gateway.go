package checks

import (
	"github.com/charlesfeval/brown/internal/platform"
)

// GatewayCheck verifies that a default gateway is configured.
type GatewayCheck struct{}

func (c *GatewayCheck) Name() string { return "Default Gateway" }

func (c *GatewayCheck) Run() Result {
	gw, err := platform.DefaultGateway()
	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: "could not determine gateway: " + err.Error()}
	}
	if gw == "" {
		return Result{Name: c.Name(), Status: Fail, Message: "no default gateway found"}
	}
	return Result{Name: c.Name(), Status: OK, Message: gw}
}
