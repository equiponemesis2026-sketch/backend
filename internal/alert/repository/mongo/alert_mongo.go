package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

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

// FindActiveByUserID devuelve la alerta activa más reciente del usuario.
func (r *alertRepository) FindActiveByUserID(ctx context.Context, userID string) (*domain.Alert, error) {
	filter := bson.M{
		"user_id": userID,
		"status":  domain.AlertStatusActive,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var alert domain.Alert
	err := r.coll.FindOne(ctx, filter, opts).Decode(&alert)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// FindByUserID lista las alertas propias del usuario, de la más reciente a la
// más antigua (historial del panel web de la víctima).
func (r *alertRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.Alert, error) {
	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(50)

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	alerts := []*domain.Alert{}
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

// Resolve marca la alerta como resuelta.
func (r *alertRepository) Resolve(ctx context.Context, id string) error {
	update := bson.M{"$set": bson.M{
		"status":      domain.AlertStatusResolved,
		"resolved_at": time.Now().UTC(),
	}}
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// SetType actualiza el tipo de la alerta (p.ej. escalar sos -> coercion).
func (r *alertRepository) SetType(ctx context.Context, id string, alertType domain.AlertType) error {
	update := bson.M{"$set": bson.M{"type": alertType}}
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// UpdateRiskMetrics actualiza las métricas de riesgo del incidente según el
// análisis de tensión vocal.
func (r *alertRepository) UpdateRiskMetrics(ctx context.Context, id string, score float64, distress bool) error {
	update := bson.M{"$set": bson.M{
		"risk_score":        score,
		"distress_detected": distress,
	}}
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
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

	alerts := []*domain.Alert{}
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}
