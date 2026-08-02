package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
)

type contactRepository struct {
	coll *mongo.Collection
}

func NewContactRepository(db *mongo.Database) *contactRepository {
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
	defer func() { _ = cursor.Close(ctx) }()

	var contacts []*domain.Contact
	if err := cursor.All(ctx, &contacts); err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *contactRepository) FindAllByLinkedUserID(ctx context.Context, linkedUserID string) ([]*domain.Contact, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"linked_user_id": linkedUserID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var contacts []*domain.Contact
	if err := cursor.All(ctx, &contacts); err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *contactRepository) FindAllPendingByLinkedUserID(ctx context.Context, linkedUserID string) ([]*domain.Contact, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"linked_user_id": linkedUserID, "is_verified": false})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var contacts []*domain.Contact
	if err := cursor.All(ctx, &contacts); err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *contactRepository) FindByIDForLinkedUser(ctx context.Context, id string, linkedUserID string) (*domain.Contact, error) {
	var contact domain.Contact
	err := r.coll.FindOne(ctx, bson.M{"_id": id, "linked_user_id": linkedUserID}).Decode(&contact)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *contactRepository) SetVerified(ctx context.Context, contactID string, linkedUserID string, verified bool) (bool, error) {
	filter := bson.M{"_id": contactID, "linked_user_id": linkedUserID}
	update := bson.M{"$set": bson.M{"is_verified": verified, "updated_at": time.Now().UTC()}}
	res, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

func (r *contactRepository) UnlinkContact(ctx context.Context, contactID string, linkedUserID string) error {
	filter := bson.M{"_id": contactID, "linked_user_id": linkedUserID}
	update := bson.M{"$set": bson.M{"is_verified": false, "updated_at": time.Now().UTC()}, "$unset": bson.M{"linked_user_id": ""}}
	_, err := r.coll.UpdateOne(ctx, filter, update)
	return err
}

func (r *contactRepository) LinkContact(ctx context.Context, contactID string, userID string, linkedUserID string) error {
	filter := bson.M{"_id": contactID, "user_id": userID}
	update := bson.M{"$set": bson.M{"linked_user_id": linkedUserID, "updated_at": time.Now().UTC()}}
	_, err := r.coll.UpdateOne(ctx, filter, update)
	return err
}

func (r *contactRepository) LinkPendingContacts(ctx context.Context, email string, phone string, userID string) error {
	filter := bson.M{
		"linked_user_id": bson.M{"$exists": false},
		"$or": []bson.M{
			{"email": email},
			{"phone": phone},
		},
	}
	update := bson.M{"$set": bson.M{"linked_user_id": userID, "updated_at": time.Now().UTC()}}
	_, err := r.coll.UpdateMany(ctx, filter, update)
	return err
}

func (r *contactRepository) Update(ctx context.Context, contact *domain.Contact) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": contact.ID, "user_id": contact.UserID}, contact)
	return err
}

func (r *contactRepository) Delete(ctx context.Context, id string, userID string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID})
	return err
}

// IsObserverVerified devuelve true si `observerID` tiene un contacto
// verificado con la víctima `victimID` (usado por el reporte forense).
func (r *contactRepository) IsObserverVerified(ctx context.Context, observerID string, victimID string) (bool, error) {
	filter := bson.M{
		"linked_user_id": observerID,
		"user_id":        victimID,
		"is_verified":    true,
	}
	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
