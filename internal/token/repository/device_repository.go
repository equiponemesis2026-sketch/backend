package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/nemesis-project/api-nemesis/internal/token/domain"
)

type deviceRepository struct {
	pairingCodesCollection *mongo.Collection
	devicesCollection      *mongo.Collection
}

func NewDeviceRepository(db *mongo.Database) domain.DeviceRepository {
	return &deviceRepository{
		pairingCodesCollection: db.Collection("device_pairing_codes"),
		devicesCollection:      db.Collection("devices"),
	}
}

func (r *deviceRepository) FindByPairingCode(ctx context.Context, code string) (*domain.PairingCode, error) {
	var pc domain.PairingCode
	err := r.pairingCodesCollection.FindOne(ctx, bson.M{"code": code}).Decode(&pc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pc, nil
}

func (r *deviceRepository) FindByID(ctx context.Context, id string) (*domain.Device, error) {
	var device domain.Device
	err := r.devicesCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&device)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *deviceRepository) Save(ctx context.Context, device *domain.Device) error {
	_, err := r.devicesCollection.InsertOne(ctx, device)
	return err
}

func (r *deviceRepository) SavePairingCode(ctx context.Context, code *domain.PairingCode) error {
	_, err := r.pairingCodesCollection.InsertOne(ctx, code)
	return err
}