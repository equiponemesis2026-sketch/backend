package usecase

import (
	"context"
	"errors"
	"testing"

	aiDomain "github.com/nemesis-project/api-nemesis/internal/ai_processing/domain"
	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
	"github.com/nemesis-project/api-nemesis/internal/audio/domain"
)

type fakeChunkRepo struct {
	chunks  map[string]*domain.AudioChunk
	created []*domain.AudioChunk
	updates int
	lastUpd *domain.AudioChunk
}

func newFakeChunkRepo() *fakeChunkRepo {
	return &fakeChunkRepo{chunks: make(map[string]*domain.AudioChunk)}
}

func (f *fakeChunkRepo) Create(_ context.Context, c *domain.AudioChunk) error {
	f.chunks[c.ID] = c
	f.created = append(f.created, c)
	return nil
}

func (f *fakeChunkRepo) FindByAlertAndIndex(_ context.Context, alertID string, idx int) (*domain.AudioChunk, error) {
	for _, c := range f.chunks {
		if c.AlertID == alertID && c.ChunkIndex == idx {
			return c, nil
		}
	}
	return nil, nil
}

func (f *fakeChunkRepo) CountByAlertID(_ context.Context, _ string) (int64, error) { return 0, nil }
func (f *fakeChunkRepo) UpdateAnalysisResult(_ context.Context, id string, score float64, conf float64, distress bool, emotion string) error {
	f.updates++
	f.lastUpd = &domain.AudioChunk{ID: id, StressScore: &score, Confidence: &conf, DistressDetected: distress, PrimaryEmotion: emotion}
	return nil
}
func (f *fakeChunkRepo) AcousticSummary(_ context.Context, _ string) (*domain.AcousticSummary, error) {
	return &domain.AcousticSummary{EmotionalBreakdown: map[string]int{}}, nil
}

type fakeAlertLookup struct {
	alert *alertDomain.Alert
}

func (f *fakeAlertLookup) FindByID(_ context.Context, _ string) (*alertDomain.Alert, error) {
	return f.alert, nil
}

type fakeRiskUpdater struct {
	calls int
}

func (f *fakeRiskUpdater) UpdateRiskMetrics(_ context.Context, _ string, _ float64, _ bool) error {
	f.calls++
	return nil
}

type fakeAnalyzer struct {
	result aiDomain.VocalTensionResult
	err    error
}

func (f *fakeAnalyzer) Analyze(_ context.Context, _ string, _ []byte) (aiDomain.VocalTensionResult, error) {
	return f.result, f.err
}

func newTestAudioUC(chunkRepo *fakeChunkRepo, alert *alertDomain.Alert, analyzer *fakeAnalyzer) (domain.AudioUseCase, *fakeChunkRepo, *fakeRiskUpdater) {
	risk := &fakeRiskUpdater{}
	uc := NewAudioUseCase(chunkRepo, &fakeAlertLookup{alert: alert}, risk, analyzer, 2, 1<<20)
	return uc, chunkRepo, risk
}

func TestStreamChunk_OK(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	uc, _, _ := newTestAudioUC(chunkRepo, alert, &fakeAnalyzer{result: aiDomain.VocalTensionResult{StressScore: 0.5}})

	err := uc.StreamChunk(context.Background(), "usr_v", &domain.StreamChunkInput{
		AlertID:    "alt_1",
		ChunkIndex: 0,
		Format:     domain.AudioFormatWAV,
		AudioData:  []byte{1, 2, 3, 4},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunkRepo.created) != 1 {
		t.Fatalf("expected 1 chunk stored, got %d", len(chunkRepo.created))
	}
}

func TestStreamChunk_AlertNotFound(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	uc, _, _ := newTestAudioUC(chunkRepo, nil, &fakeAnalyzer{})

	err := uc.StreamChunk(context.Background(), "usr_v", &domain.StreamChunkInput{
		AlertID: "alt_nope", ChunkIndex: 0, AudioData: []byte{1, 2},
	})
	if !errors.Is(err, ErrAlertNotFound) {
		t.Fatalf("expected ErrAlertNotFound, got %v", err)
	}
}

func TestStreamChunk_ForbiddenForOtherUser(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_other", Status: alertDomain.AlertStatusResolved}
	uc, _, _ := newTestAudioUC(chunkRepo, alert, &fakeAnalyzer{})

	err := uc.StreamChunk(context.Background(), "usr_v", &domain.StreamChunkInput{
		AlertID: "alt_1", ChunkIndex: 0, AudioData: []byte{1, 2},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestStreamChunk_OtherUserActiveAlertForbidden fija la regresión de seguridad:
// un tercero NO debe poder inyectar audio en la alerta activa de otra víctima.
func TestStreamChunk_OtherUserActiveAlertForbidden(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_other", Status: alertDomain.AlertStatusActive}
	uc, _, _ := newTestAudioUC(chunkRepo, alert, &fakeAnalyzer{})

	err := uc.StreamChunk(context.Background(), "usr_v", &domain.StreamChunkInput{
		AlertID: "alt_1", ChunkIndex: 0, AudioData: []byte{1, 2},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for active alert of another user, got %v", err)
	}
}

func TestStreamChunk_Duplicate(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	uc, _, _ := newTestAudioUC(chunkRepo, alert, &fakeAnalyzer{})

	in := &domain.StreamChunkInput{AlertID: "alt_1", ChunkIndex: 1, AudioData: []byte{1, 2}}
	if err := uc.StreamChunk(context.Background(), "usr_v", in); err != nil {
		t.Fatalf("first chunk failed: %v", err)
	}
	if err := uc.StreamChunk(context.Background(), "usr_v", in); !errors.Is(err, ErrDuplicateChunk) {
		t.Fatalf("expected ErrDuplicateChunk, got %v", err)
	}
}

func TestStreamChunk_UnsupportedFormat(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	uc, _, _ := newTestAudioUC(chunkRepo, alert, &fakeAnalyzer{})

	err := uc.StreamChunk(context.Background(), "usr_v", &domain.StreamChunkInput{
		AlertID: "alt_1", ChunkIndex: 0, Format: domain.AudioFormat("opus"), AudioData: []byte{1, 2},
	})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestStreamChunk_TooLarge(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	uc, _, _ := newTestAudioUC(chunkRepo, alert, &fakeAnalyzer{})

	big := make([]byte, 2<<20)
	err := uc.StreamChunk(context.Background(), "usr_v", &domain.StreamChunkInput{
		AlertID: "alt_1", ChunkIndex: 0, AudioData: big,
	})
	if !errors.Is(err, ErrAudioTooLarge) {
		t.Fatalf("expected ErrAudioTooLarge, got %v", err)
	}
}
