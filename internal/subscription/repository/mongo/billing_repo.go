package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/nemesis-project/api-nemesis/internal/subscription/domain"
)

type subscriptionRepository struct {
	coll *mongo.Collection
}

func NewSubscriptionRepository(db *mongo.Database) domain.SubscriptionRepository {
	return &subscriptionRepository{
		coll: db.Collection("subscriptions"),
	}
}

func (r *subscriptionRepository) Upsert(ctx context.Context, sub *domain.Subscription) error {
	opts := options.Replace().SetUpsert(true)
	_, err := r.coll.ReplaceOne(ctx, bson.M{"user_id": sub.UserID}, sub, opts)
	return err
}

func (r *subscriptionRepository) FindByUserID(ctx context.Context, userID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.coll.FindOne(ctx, bson.M{"user_id": userID}).Decode(&sub)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *subscriptionRepository) FindByStripeCustomerID(ctx context.Context, customerID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.coll.FindOne(ctx, bson.M{"stripe_customer_id": customerID}).Decode(&sub)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}