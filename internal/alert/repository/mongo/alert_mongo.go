package mongo

import (
	"context"
	"github.com/nemesis-back/internal/alert/domain"
	"go.mongodb.org/mongo-driver/mongo"
)

type alertMongoRepository struct {
	db         *mongo.Database
	collection string
}

func NewAlertMongoRepository(db *mongo.Database) domain.AlertRepository {
	return &alertMongoRepository{
		db:         db,
		collection: "alerts",
	}
}

func (r *alertMongoRepository) Create(ctx context.Context, alert *domain.Alert) error {
	// TODO: Implement mongo insert
	return nil
}

func (r *alertMongoRepository) GetByID(ctx context.Context, id string) (*domain.Alert, error) {
	// TODO: Implement mongo find
	return nil, nil
}

func (r *alertMongoRepository) Update(ctx context.Context, alert *domain.Alert) error {
	// TODO: Implement mongo update
	return nil
}
