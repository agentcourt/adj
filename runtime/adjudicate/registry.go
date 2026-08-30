package adjudicate

import (
	"context"
	"fmt"
)

type ProcedureRunner interface {
	Run(context.Context, ProcedureRequest) (ProcedureOutcome, error)
}

type Registration struct {
	Capabilities Capabilities
	Runner       ProcedureRunner
}

type Registry struct {
	registrations map[Procedure]Registration
}

func NewRegistry(registrations map[Procedure]Registration) (Registry, error) {
	copy := make(map[Procedure]Registration, len(registrations))
	for procedure, registration := range registrations {
		if !procedure.Valid() {
			return Registry{}, fmt.Errorf("invalid registered procedure %q", procedure)
		}
		if registration.Runner == nil {
			return Registry{}, fmt.Errorf("procedure %q has no runner", procedure)
		}
		copy[procedure] = registration
	}
	return Registry{registrations: copy}, nil
}

func (r Registry) Lookup(procedure Procedure) (Registration, bool) {
	registration, ok := r.registrations[procedure]
	return registration, ok
}
