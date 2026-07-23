package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
)

type contactRepository struct {
	coll *mongo.Collection
}

func NewContactRepository(db *mongo.Database) domain.ContactRepository {
	return &contactRepository{
		coll: db.Collection("contacts"),
	}
}

func (r *contactRepository) Create(ctx context.Context, contact *domain.Contact) error {
	_, err := r.coll.InsertOne(ctx, contact)
	return err
}

func (r *contactRepository) FindByID(ctx context.Context, id string, userID string) (*domain.Contact, error) {
	var contact domain.Contact
	err := r.coll.FindOne(ctx, bson.M{"_id": id, "user_id": userID}).Decode(&contact)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *contactRepository) FindAllByUserID(ctx context.Context, userID string) ([]*domain.Contact, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var contacts []*domain.Contact
	if err := cursor.All(ctx, &contacts); err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *contactRepository) Update(ctx context.Context, contact *domain.Contact) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": contact.ID, "user_id": contact.UserID}, contact)
	return err
}

func (r *contactRepository) Delete(ctx context.Context, id string, userID string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID})
	return err
}
