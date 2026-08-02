package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
)

type alertRepository struct {
	coll *mongo.Collection
}

func NewAlertRepository(db *mongo.Database) domain.AlertRepository {
	return &alertRepository{
		coll: db.Collection("alerts"),
	}
}

func (r *alertRepository) Create(ctx context.Context, alert *domain.Alert) error {
	_, err := r.coll.InsertOne(ctx, alert)
	return err
}

func (r *alertRepository) FindByID(ctx context.Context, id string) (*domain.Alert, error) {
	var alert domain.Alert
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&alert)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

func (r *alertRepository) FindActiveByUserIDs(ctx context.Context, userIDs []string) ([]*domain.Alert, error) {
	if len(userIDs) == 0 {
		return []*domain.Alert{}, nil
	}

	filter := bson.M{
		"user_id": bson.M{"$in": userIDs},
		"status":  domain.AlertStatusActive,
	}
	cursor, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var alerts []*domain.Alert
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}
