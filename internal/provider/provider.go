package provider

import (
	"context"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

type Provider interface {
	Name() string
	Search(ctx context.Context, req model.SearchRequest) ([]model.Flight, error)
}
