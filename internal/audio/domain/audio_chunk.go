package domain

import (
	"context"
	"time"

	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
)

// AudioFormat identifica el códec del fragmento de audio entrante.
type AudioFormat string

const (
	AudioFormatPCM AudioFormat = "pcm"
	AudioFormatWAV AudioFormat = "wav"
)

// AudioChunk es un fragmento de audio de una alerta activa, con las métricas
// de tensión vocal anotadas tras el análisis asíncrono.
type AudioChunk struct {
	ID               string      `json:"id" bson:"_id"`
	AlertID          string      `json:"alert_id" bson:"alert_id"`
	UserID           string      `json:"user_id" bson:"user_id"`
	ChunkIndex       int         `json:"chunk_index" bson:"chunk_index"`
	Format           AudioFormat `json:"format" bson:"format"`
	DurationMS       int         `json:"duration_ms" bson:"duration_ms"`
	Timestamp        int64       `json:"timestamp" bson:"timestamp"`
	AudioData        []byte      `json:"-" bson:"audio_data"`
	StressScore      *float64    `json:"stress_score,omitempty" bson:"stress_score,omitempty"`
	Confidence       *float64    `json:"confidence,omitempty" bson:"confidence,omitempty"`
	DistressDetected bool        `json:"distress_detected" bson:"distress_detected"`
	PrimaryEmotion   string      `json:"primary_emotion,omitempty" bson:"primary_emotion,omitempty"`
	StoredAt         time.Time   `json:"stored_at" bson:"stored_at"`
}

// StreamChunkInput encapsula el DTO de recepción de un fragmento de audio.
type StreamChunkInput struct {
	AlertID    string
	ChunkIndex int
	Format     AudioFormat
	AudioData  []byte
	DurationMS int
	Timestamp  int64
}

// AcousticSummary resume el procesamiento acústico Wav2Vec2 de una alerta
// para el reporte forense.
type AcousticSummary struct {
	TotalChunks        int            `json:"total_chunks"`
	AvgStress          float64        `json:"avg_stress"`
	PeakStress         float64        `json:"peak_stress"`
	DistressAlerts     int            `json:"distress_alerts"`
	EmotionalBreakdown map[string]int `json:"emotional_breakdown"`
}

// AudioChunkRepository define el contrato de persistencia de fragmentos.
type AudioChunkRepository interface {
	Create(ctx context.Context, chunk *AudioChunk) error
	FindByAlertAndIndex(ctx context.Context, alertID string, index int) (*AudioChunk, error)
	CountByAlertID(ctx context.Context, alertID string) (int64, error)
	UpdateAnalysisResult(ctx context.Context, id string, score float64, confidence float64, distress bool, emotion string) error
	AcousticSummary(ctx context.Context, alertID string) (*AcousticSummary, error)
}

// AudioUseCase define la lógica de negocio de la ingesta de audio en vivo.
type AudioUseCase interface {
	StreamChunk(ctx context.Context, userID string, input *StreamChunkInput) error
}

// RiskUpdater actualiza las métricas de riesgo del incidente. La implementa
// el repositorio de alertas para evitar acoplar el módulo de audio a él.
type RiskUpdater interface {
	UpdateRiskMetrics(ctx context.Context, id string, score float64, distress bool) error
}

// AlertLookup permite validar que la alerta existe y pertenece al usuario.
// Lo implementa el repositorio de alertas.
type AlertLookup interface {
	FindByID(ctx context.Context, id string) (*alertDomain.Alert, error)
}
