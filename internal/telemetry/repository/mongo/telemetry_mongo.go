package mongo

import (
	"context"
	"github.com/nemesis-back/internal/telemetry/domain"
	"go.mongodb.org/mongo-driver/mongo"
)

type telemetryMongoRepository struct {
	db         *mongo.Database
	collection string
}

func NewTelemetryMongoRepository(db *mongo.Database) domain.TelemetryRepository {
	return &telemetryMongoRepository{
		db:         db,
		collection: "telemetry", // Recommended to be structured as a MongoDB Time-Series collection
	}
}

func (r *telemetryMongoRepository) Store(ctx context.Context, telemetry *domain.Telemetry) error {
	// TODO: Implement telemetry insert
	return nil
}

func (r *telemetryMongoRepository) GetLatestByUserID(ctx context.Context, userID string) (*domain.Telemetry, error) {
	// TODO: Implement get latest telemetry
	return nil, nil
}
