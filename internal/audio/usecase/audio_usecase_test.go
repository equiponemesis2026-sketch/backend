package usecase

import (
	"context"
	"errors"
	"sort"
	"testing"

	aiDomain "github.com/nemesis-project/api-nemesis/internal/ai_processing/domain"
	aiServices "github.com/nemesis-project/api-nemesis/internal/ai_processing/services"
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
	if c, ok := f.chunks[id]; ok {
		c.StressScore = &score
		c.Confidence = &conf
		c.DistressDetected = distress
		c.PrimaryEmotion = emotion
		f.lastUpd = c
	}
	return nil
}
func (f *fakeChunkRepo) AcousticSummary(_ context.Context, _ string) (*domain.AcousticSummary, error) {
	return &domain.AcousticSummary{EmotionalBreakdown: map[string]int{}}, nil
}
func (f *fakeChunkRepo) FindRecentByAlertID(_ context.Context, alertID string, limit int) ([]*domain.AudioChunk, error) {
	var matched []*domain.AudioChunk
	for _, c := range f.chunks {
		if c.AlertID == alertID {
			matched = append(matched, c)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ChunkIndex > matched[j].ChunkIndex })
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
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
	return newTestAudioUCWithConsecutive(chunkRepo, alert, analyzer, 1)
}

func newTestAudioUCWithConsecutive(chunkRepo *fakeChunkRepo, alert *alertDomain.Alert, analyzer *fakeAnalyzer, requiredConsecutive int) (domain.AudioUseCase, *fakeChunkRepo, *fakeRiskUpdater) {
	risk := &fakeRiskUpdater{}
	uc := NewAudioUseCase(chunkRepo, &fakeAlertLookup{alert: alert}, risk, analyzer, 2, 1<<20, requiredConsecutive)
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

// seedChunk inserta un chunk ya analizado directamente en el fake repo, sin
// pasar por StreamChunk, para armar el historial que confirmedByConsecutiveChunks
// va a leer.
func seedChunk(repo *fakeChunkRepo, alertID string, index int, distress bool) {
	repo.chunks["seed_"+alertID+"_"+string(rune('0'+index))] = &domain.AudioChunk{
		ID: "seed_" + alertID + "_" + string(rune('0'+index)), AlertID: alertID, ChunkIndex: index, DistressDetected: distress,
	}
}

// TestAnalyzeJob_SingleChunk_NotEnoughToConfirm fija que un solo chunk en
// distress no alcanza para escalar el riesgo si se exigen varios seguidos:
// evita que un chunk aislado (ruido mal clasificado) dispare un falso crítico.
func TestAnalyzeJob_SingleChunk_NotEnoughToConfirm(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	analyzer := &fakeAnalyzer{result: aiDomain.VocalTensionResult{StressScore: 0.9, DistressDetected: true, PrimaryEmotion: "miedo"}}
	uc, chunkRepo, risk := newTestAudioUCWithConsecutive(chunkRepo, alert, analyzer, 3)

	seedChunk(chunkRepo, "alt_1", 0, true)
	audioUC := uc.(*audioUseCase)
	audioUC.analyzeJob(context.Background(), aiServices.Job{AlertID: "alt_1", ChunkID: "seed_alt_1_0", Format: "wav", Audio: []byte{1}})

	if risk.calls != 0 {
		t.Errorf("expected no risk escalation with only 1 distress chunk (need 3), got %d calls", risk.calls)
	}
}

// TestAnalyzeJob_ConsecutiveChunksConfirm fija que 3 chunks seguidos en
// distress sí escalan el riesgo del incidente.
func TestAnalyzeJob_ConsecutiveChunksConfirm(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	analyzer := &fakeAnalyzer{result: aiDomain.VocalTensionResult{StressScore: 0.9, DistressDetected: true, PrimaryEmotion: "miedo"}}
	uc, chunkRepo, risk := newTestAudioUCWithConsecutive(chunkRepo, alert, analyzer, 3)

	seedChunk(chunkRepo, "alt_1", 0, true)
	seedChunk(chunkRepo, "alt_1", 1, true)
	seedChunk(chunkRepo, "alt_1", 2, true)
	audioUC := uc.(*audioUseCase)
	audioUC.analyzeJob(context.Background(), aiServices.Job{AlertID: "alt_1", ChunkID: "seed_alt_1_2", Format: "wav", Audio: []byte{1}})

	if risk.calls != 1 {
		t.Errorf("expected risk escalation after 3 consecutive distress chunks, got %d calls", risk.calls)
	}
}

// TestAnalyzeJob_BrokenStreak_DoesNotConfirm fija que un chunk sin distress
// en medio de la ventana reciente rompe la confirmación.
func TestAnalyzeJob_BrokenStreak_DoesNotConfirm(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	analyzer := &fakeAnalyzer{result: aiDomain.VocalTensionResult{StressScore: 0.9, DistressDetected: true, PrimaryEmotion: "miedo"}}
	uc, chunkRepo, risk := newTestAudioUCWithConsecutive(chunkRepo, alert, analyzer, 3)

	seedChunk(chunkRepo, "alt_1", 0, false) // chunk previo sin distress
	seedChunk(chunkRepo, "alt_1", 1, true)
	seedChunk(chunkRepo, "alt_1", 2, true)
	audioUC := uc.(*audioUseCase)
	audioUC.analyzeJob(context.Background(), aiServices.Job{AlertID: "alt_1", ChunkID: "seed_alt_1_2", Format: "wav", Audio: []byte{1}})

	if risk.calls != 0 {
		t.Errorf("expected no escalation when the streak is broken, got %d calls", risk.calls)
	}
}

// TestAnalyzeJob_RequiredConsecutiveOne_ConfirmsImmediately confirma que
// con requiredConsecutive<=1 el comportamiento es el de antes: cada chunk
// decide por sí solo, sin esperar confirmación.
func TestAnalyzeJob_RequiredConsecutiveOne_ConfirmsImmediately(t *testing.T) {
	chunkRepo := newFakeChunkRepo()
	alert := &alertDomain.Alert{ID: "alt_1", UserID: "usr_v", Status: alertDomain.AlertStatusActive}
	analyzer := &fakeAnalyzer{result: aiDomain.VocalTensionResult{StressScore: 0.9, DistressDetected: true, PrimaryEmotion: "miedo"}}
	uc, chunkRepo, risk := newTestAudioUCWithConsecutive(chunkRepo, alert, analyzer, 1)

	seedChunk(chunkRepo, "alt_1", 0, true)
	audioUC := uc.(*audioUseCase)
	audioUC.analyzeJob(context.Background(), aiServices.Job{AlertID: "alt_1", ChunkID: "seed_alt_1_0", Format: "wav", Audio: []byte{1}})

	if risk.calls != 1 {
		t.Errorf("expected immediate escalation with requiredConsecutive=1, got %d calls", risk.calls)
	}
}
