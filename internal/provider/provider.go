package provider

import (
	"context"
	"foghorn/internal/model"
)

// Provider is the interface all alert sources must implement.
type Provider interface {
	Name() string
	Type() string
	SupportsSilence() bool
	Fetch(ctx context.Context) ([]model.Alert, error)
	Silence(ctx context.Context, req model.SilenceRequest) (string, error)
	Unsilence(ctx context.Context, silenceID string) error
	Health(ctx context.Context) model.ProviderHealth
}

// SilenceProvider is implemented by sources that can fetch silence details.
type SilenceProvider interface {
	FetchSilences(ctx context.Context) ([]model.SilenceInfo, error)
}

// OnCallProvider is implemented by sources that can expose current on-call users.
type OnCallProvider interface {
	FetchOnCall(ctx context.Context) (*model.OnCallStatus, error)
}
