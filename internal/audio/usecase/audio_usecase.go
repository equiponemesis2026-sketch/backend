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
	chunkRepo           domain.AudioChunkRepository
	alertLookup         domain.AlertLookup
	riskUpdater         domain.RiskUpdater
	analyzer            aiDomain.VocalTensionAnalyzer
	pool                *aiServices.Pool
	maxChunkBytes       int
	requiredConsecutive int
}

// NewAudioUseCase crea el caso de uso de ingesta de audio. requiredConsecutive
// es cuántos chunks seguidos deben marcar distress antes de escalar el
// riesgo del incidente (<=1 desactiva la confirmación: cada chunk decide
// por sí solo). Ver domain.DefaultRequiredConsecutiveDistress.
func NewAudioUseCase(
	chunkRepo domain.AudioChunkRepository,
	alertLookup domain.AlertLookup,
	riskUpdater domain.RiskUpdater,
	analyzer aiDomain.VocalTensionAnalyzer,
	workerCount int,
	maxChunkBytes int,
	requiredConsecutive int,
) domain.AudioUseCase {
	uc := &audioUseCase{
		chunkRepo:           chunkRepo,
		alertLookup:         alertLookup,
		riskUpdater:         riskUpdater,
		analyzer:            analyzer,
		maxChunkBytes:       maxChunkBytes,
		requiredConsecutive: requiredConsecutive,
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

// analyzeJob procesa un fragmento y, si el distress se confirma en varios
// chunks seguidos, escala el riesgo del incidente. Best-effort: los fallos
// solo se loguean.
func (uc *audioUseCase) analyzeJob(ctx context.Context, job aiServices.Job) {
	result, err := uc.analyzer.Analyze(ctx, job.Format, job.Audio)
	if err != nil {
		slog.Warn("audio: analysis failed", "chunk_id", job.ChunkID, "error", err)
		return
	}

	if err := uc.chunkRepo.UpdateAnalysisResult(ctx, job.ChunkID, result.StressScore, result.Confidence, result.DistressDetected, result.PrimaryEmotion); err != nil {
		slog.Warn("audio: failed to persist analysis result", "chunk_id", job.ChunkID, "error", err)
	}

	if !result.DistressDetected {
		return
	}

	confirmed, err := uc.confirmedByConsecutiveChunks(ctx, job.AlertID)
	if err != nil {
		slog.Warn("audio: failed to check consecutive distress chunks", "alert_id", job.AlertID, "error", err)
		return
	}

	slog.Warn("audio: distress detected in chunk", "chunk_id", job.ChunkID, "alert_id", job.AlertID, "stress_score", result.StressScore, "emotion", result.PrimaryEmotion, "confirmed", confirmed)

	if !confirmed {
		return
	}

	if err := uc.riskUpdater.UpdateRiskMetrics(ctx, job.AlertID, result.StressScore, true); err != nil {
		slog.Warn("audio: failed to update incident risk", "alert_id", job.AlertID, "error", err)
	}
}

// confirmedByConsecutiveChunks evita escalar el riesgo por un solo chunk
// atípico: exige que los últimos `requiredConsecutive` chunks analizados de
// la alerta hayan marcado distress. El clasificador no distingue "esto no
// es voz" de una emoción real (silencio puro llegó a marcar 93% "enojo" en
// pruebas); exigir varios chunks seguidos reduce mucho ese riesgo sin
// retrasar el análisis individual de cada chunk, que sigue siendo al instante.
func (uc *audioUseCase) confirmedByConsecutiveChunks(ctx context.Context, alertID string) (bool, error) {
	if uc.requiredConsecutive <= 1 {
		return true, nil
	}
	recent, err := uc.chunkRepo.FindRecentByAlertID(ctx, alertID, uc.requiredConsecutive)
	if err != nil {
		return false, err
	}
	if len(recent) < uc.requiredConsecutive {
		return false, nil
	}
	for _, c := range recent {
		if !c.DistressDetected {
			return false, nil
		}
	}
	return true, nil
}
