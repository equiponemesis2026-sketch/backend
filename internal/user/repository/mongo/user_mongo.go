package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/nemesis-project/api-nemesis/internal/user/domain"
)

type userRepository struct {
	coll *mongo.Collection
}

// NewUserRepository crea un repositorio de usuarios con la colección de MongoDB.
func NewUserRepository(db *mongo.Database) domain.UserRepository {
	return &userRepository{
		coll: db.Collection("users"),
	}
}

// Create inserta un nuevo usuario en MongoDB.
func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.coll.InsertOne(ctx, user)
	return err
}

// FindByEmail busca un usuario por su email. Retorna nil si no existe.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.coll.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByPhone busca un usuario por su teléfono. Retorna nil si no existe.
func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var user domain.User
	err := r.coll.FindOne(ctx, bson.M{"phone": phone}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID busca un usuario por su ID de negocio (ej. usr_<uuid>). Retorna nil si no existe.
func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
