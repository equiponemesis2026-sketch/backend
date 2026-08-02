package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/nemesis-project/api-nemesis/internal/evidence/domain"
)

type evidenceReportRepository struct {
	coll *mongo.Collection
}

// NewEvidenceReportRepository crea el repositorio de la colección evidence_reports.
func NewEvidenceReportRepository(db *mongo.Database) domain.EvidenceReportRepository {
	return &evidenceReportRepository{
		coll: db.Collection("evidence_reports"),
	}
}

// Save inserta el expediente. Los expedientes son inmutables: si el alert_id
// ya existe (p.ej. carrera entre dos solicitudes concurrentes), no se reescribe
// y se devuelve ErrReportAlreadyExists.
func (r *evidenceReportRepository) Save(ctx context.Context, report *domain.EvidenceReport) error {
	_, err := r.coll.InsertOne(ctx, report)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrReportAlreadyExists
	}
	return err
}

func (r *evidenceReportRepository) FindByID(ctx context.Context, alertID string) (*domain.EvidenceReport, error) {
	var report domain.EvidenceReport
	err := r.coll.FindOne(ctx, bson.M{"_id": alertID}).Decode(&report)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}
