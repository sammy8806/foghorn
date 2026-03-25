package provider

import (
	"context"
	"foghorn/internal/model"
)

// Provider is the interface all alert sources must implement.
type Provider interface {
	Name() string
	Type() string
	Fetch(ctx context.Context) ([]model.Alert, error)
	Silence(ctx context.Context, req model.SilenceRequest) (string, error)
	Unsilence(ctx context.Context, silenceID string) error
	Health(ctx context.Context) model.ProviderHealth
}
