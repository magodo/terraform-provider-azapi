package framework

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/clients"
)

type Meta struct {
	ResourceClient *clients.ResourceClient
	Option         *clients.Option
}

type Logger interface {
	Debug(ctx context.Context, msg string, additionalFields ...map[string]any)
	Info(ctx context.Context, msg string, additionalFields ...map[string]any)
	Warn(ctx context.Context, msg string, additionalFields ...map[string]any)
	Error(ctx context.Context, msg string, additionalFields ...map[string]any)
}
