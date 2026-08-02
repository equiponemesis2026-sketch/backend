package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	aiDomain "github.com/nemesis-project/api-nemesis/internal/ai_processing/domain"
	aiServices "github.com/nemesis-project/api-nemesis/internal/ai_processing/services"
	"github.com/nemesis-project/api-nemesis/internal/audio/domain"
)

var (
	ErrAlertNotFound     = errors.New("alert not found")
	ErrForbidden         = errors.New("forbidden")
	ErrDuplicateChunk    = errors.New("chunk already received")
	ErrUnsupportedFormat = errors.New("unsupported audio format")
	ErrInvalidChunk      = errors.New("invalid chunk")
	ErrAudioTooLarge     = errors.New("audio chunk exceeds size limit")
)

// AudioUseCase impl la ingesta de fragmentos de audio en tiempo real.
type audioUseCase struct {
	chunkRepo     domain.AudioChunkRepository
	alertLookup   domain.AlertLookup
	riskUpdater   domain.RiskUpdater
	analyzer      aiDomain.VocalTensionAnalyzer
	pool          *aiServices.Pool
	maxChunkBytes int
}

// NewAudioUseCase crea el caso de uso de ingesta de audio.
func NewAudioUseCase(
	chunkRepo domain.AudioChunkRepository,
	alertLookup domain.AlertLookup,
	riskUpdater domain.RiskUpdater,
	analyzer aiDomain.VocalTensionAnalyzer,
	workerCount int,
	maxChunkBytes int,
) domain.AudioUseCase {
	uc := &audioUseCase{
		chunkRepo:     chunkRepo,
		alertLookup:   alertLookup,
		riskUpdater:   riskUpdater,
		analyzer:      analyzer,
		maxChunkBytes: maxChunkBytes,
	}
	if uc.maxChunkBytes <= 0 {
		uc.maxChunkBytes = 1 << 20
	}
	uc.pool = aiServices.NewPool(context.Background(), workerCount, uc.analyzeJob)
	return uc
}

// StreamChunk valida, persiste y despacha asíncronamente un fragmento de audio.
func (uc *audioUseCase) StreamChunk(ctx context.Context, userID string, input *domain.StreamChunkInput) error {
	if input == nil || input.AlertID == "" {
		return ErrInvalidChunk
	}
	if input.ChunkIndex < 0 {
		return ErrInvalidChunk
	}
	if len(input.AudioData) == 0 {
		return ErrInvalidChunk
	}
	if len(input.AudioData) > uc.maxChunkBytes {
		return ErrAudioTooLarge
	}

	switch input.Format {
	case "", domain.AudioFormatWAV:
		input.Format = domain.AudioFormatWAV
	case domain.AudioFormatPCM:
		// ok
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedFormat, input.Format)
	}

	alert, err := uc.alertLookup.FindByID(ctx, input.AlertID)
	if err != nil {
		return err
	}
	if alert == nil {
		return ErrAlertNotFound
	}
	// Solo la víctima propietaria puede inyectar audio en su incidente.
	if alert.UserID != userID {
		return ErrForbidden
	}

	existing, err := uc.chunkRepo.FindByAlertAndIndex(ctx, input.AlertID, input.ChunkIndex)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrDuplicateChunk
	}

	chunk := &domain.AudioChunk{
		ID:         uuid.NewString(),
		AlertID:    input.AlertID,
		UserID:     userID,
		ChunkIndex: input.ChunkIndex,
		Format:     input.Format,
		DurationMS: input.DurationMS,
		Timestamp:  input.Timestamp,
		AudioData:  input.AudioData,
		StoredAt:   time.Now().UTC(),
	}
	if err := uc.chunkRepo.Create(ctx, chunk); err != nil {
		return err
	}

	// Despacho asíncrono al servicio de análisis de tensión vocal.
	uc.pool.Enqueue(aiServices.Job{
		AlertID: chunk.AlertID,
		ChunkID: chunk.ID,
		Format:  string(chunk.Format),
		Audio:   chunk.AudioData,
	})
	return nil
}

// analyzeJob procesa un fragmento y actualiza las métricas de riesgo si el
// distress supera el umbral crítico. Best-effort: los fallos solo se loguean.
func (uc *audioUseCase) analyzeJob(ctx context.Context, job aiServices.Job) {
	result, err := uc.analyzer.Analyze(ctx, job.Format, job.Audio)
	if err != nil {
		slog.Warn("audio: analysis failed", "chunk_id", job.ChunkID, "error", err)
		return
	}

	if err := uc.chunkRepo.UpdateAnalysisResult(ctx, job.ChunkID, result.StressScore, result.Confidence, result.DistressDetected, result.PrimaryEmotion); err != nil {
		slog.Warn("audio: failed to persist analysis result", "chunk_id", job.ChunkID, "error", err)
	}

	if result.DistressDetected {
		slog.Warn("audio: distress detected", "chunk_id", job.ChunkID, "alert_id", job.AlertID, "stress_score", result.StressScore, "emotion", result.PrimaryEmotion)
		if err := uc.riskUpdater.UpdateRiskMetrics(ctx, job.AlertID, result.StressScore, true); err != nil {
			slog.Warn("audio: failed to update incident risk", "alert_id", job.AlertID, "error", err)
		}
	}
}
