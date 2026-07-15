package mongo

import (
	"context"
	"github.com/nemesis-back/internal/user/domain"
	"go.mongodb.org/mongo-driver/mongo"
)

type userMongoRepository struct {
	db         *mongo.Database
	collection string
}

func NewUserMongoRepository(db *mongo.Database) domain.UserRepository {
	return &userMongoRepository{
		db:         db,
		collection: "users",
	}
}

func (r *userMongoRepository) Create(ctx context.Context, user *domain.User) error {
	// TODO: Implement user creation
	return nil
}

func (r *userMongoRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	// TODO: Implement user get by id
	return nil, nil
}

func (r *userMongoRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	// TODO: Implement user get by email
	return nil, nil
}

func (r *userMongoRepository) Update(ctx context.Context, user *domain.User) error {
	// TODO: Implement user update
	return nil
}
