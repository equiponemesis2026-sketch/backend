package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/nemesis-project/api-nemesis/internal/audio/domain"
)

type audioChunkRepository struct {
	coll *mongo.Collection
}

// NewAudioChunkRepository crea el repositorio de la colección audio_chunks.
func NewAudioChunkRepository(db *mongo.Database) domain.AudioChunkRepository {
	return &audioChunkRepository{
		coll: db.Collection("audio_chunks"),
	}
}

func (r *audioChunkRepository) Create(ctx context.Context, chunk *domain.AudioChunk) error {
	_, err := r.coll.InsertOne(ctx, chunk)
	return err
}

func (r *audioChunkRepository) FindByAlertAndIndex(ctx context.Context, alertID string, index int) (*domain.AudioChunk, error) {
	var chunk domain.AudioChunk
	err := r.coll.FindOne(ctx, bson.M{"alert_id": alertID, "chunk_index": index}).Decode(&chunk)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chunk, nil
}

func (r *audioChunkRepository) CountByAlertID(ctx context.Context, alertID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"alert_id": alertID})
}

// FindRecentByAlertID devuelve los últimos `limit` chunks de la alerta,
// ordenados del más reciente al más antiguo (por chunk_index).
func (r *audioChunkRepository) FindRecentByAlertID(ctx context.Context, alertID string, limit int) ([]*domain.AudioChunk, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "chunk_index", Value: -1}}).
		SetLimit(int64(limit))
	cursor, err := r.coll.Find(ctx, bson.M{"alert_id": alertID}, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	chunks := []*domain.AudioChunk{}
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

func (r *audioChunkRepository) UpdateAnalysisResult(ctx context.Context, id string, score float64, confidence float64, distress bool, emotion string) error {
	update := bson.M{"$set": bson.M{
		"stress_score":      score,
		"confidence":        confidence,
		"distress_detected": distress,
		"primary_emotion":   emotion,
	}}
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *audioChunkRepository) AcousticSummary(ctx context.Context, alertID string) (*domain.AcousticSummary, error) {
	filter := bson.M{"alert_id": alertID}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$alert_id",
			"total":       bson.M{"$sum": 1},
			"distress":    bson.M{"$sum": bson.M{"$cond": bson.A{"$distress_detected", 1, 0}}},
			"avg_stress":  bson.M{"$avg": "$stress_score"},
			"peak_stress": bson.M{"$max": "$stress_score"},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var groups []struct {
		Total      int     `bson:"total"`
		Distress   int     `bson:"distress"`
		AvgStress  float64 `bson:"avg_stress"`
		PeakStress float64 `bson:"peak_stress"`
	}
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, err
	}

	summary := &domain.AcousticSummary{
		TotalChunks:        0,
		AvgStress:          0,
		PeakStress:         0,
		DistressAlerts:     0,
		EmotionalBreakdown: make(map[string]int),
	}
	if len(groups) == 1 {
		summary.TotalChunks = groups[0].Total
		summary.DistressAlerts = groups[0].Distress
		summary.AvgStress = groups[0].AvgStress
		summary.PeakStress = groups[0].PeakStress
	}

	// Desglose emocional
	emoPipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$primary_emotion",
			"count": bson.M{"$sum": 1},
		}}},
	}
	emoCursor, err := r.coll.Aggregate(ctx, emoPipeline)
	if err != nil {
		return summary, err
	}
	defer func() { _ = emoCursor.Close(ctx) }()

	var emotions []struct {
		Emotion string `bson:"_id"`
		Count   int    `bson:"count"`
	}
	if err := emoCursor.All(ctx, &emotions); err != nil {
		return summary, err
	}
	for _, e := range emotions {
		if e.Emotion != "" {
			summary.EmotionalBreakdown[e.Emotion] = e.Count
		}
	}

	return summary, nil
}
