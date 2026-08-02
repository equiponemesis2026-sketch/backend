package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
)

type telemetryRepository struct {
	coll *mongo.Collection
}

// NewTelemetryRepository crea el repositorio de la colección telemetry_logs.
func NewTelemetryRepository(db *mongo.Database) domain.TelemetryRepository {
	return &telemetryRepository{
		coll: db.Collection("telemetry_logs"),
	}
}

func (r *telemetryRepository) Save(ctx context.Context, record *domain.TelemetryRecord) error {
	_, err := r.coll.InsertOne(ctx, record)
	return err
}

func (r *telemetryRepository) FindByAlertID(ctx context.Context, alertID string) ([]*domain.TelemetryRecord, error) {
	filter := bson.M{"alert_id": alertID}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var records []*domain.TelemetryRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}
	return records, nil
}
