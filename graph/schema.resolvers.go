package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/chenak/hackernews/database"
	"github.com/chenak/hackernews/graph/generated"
	"github.com/chenak/hackernews/graph/model"
)

func (r *queryResolver) Crypto(ctx context.Context, name string) (*model.Crypto, error) {
	return database.Data(name), nil
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
