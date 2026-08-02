package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
	audioDomain "github.com/nemesis-project/api-nemesis/internal/audio/domain"
	"github.com/nemesis-project/api-nemesis/internal/evidence/domain"
)

type fakeEvidenceRepo struct {
	reports               map[string]*domain.EvidenceReport
	failSaveWithDuplicate bool
}

func newFakeEvidenceRepo() *fakeEvidenceRepo {
	return &fakeEvidenceRepo{reports: make(map[string]*domain.EvidenceReport)}
}
func (f *fakeEvidenceRepo) Save(_ context.Context, r *domain.EvidenceReport) error {
	if _, exists := f.reports[r.ID]; exists {
		if f.failSaveWithDuplicate {
			return domain.ErrReportAlreadyExists
		}
		return nil
	}
	f.reports[r.ID] = r
	return nil
}
func (f *fakeEvidenceRepo) FindByID(_ context.Context, id string) (*domain.EvidenceReport, error) {
	return f.reports[id], nil
}

type fakeEvAlertLookup struct {
	alert *alertDomain.Alert
}

func (f *fakeEvAlertLookup) FindByID(_ context.Context, _ string) (*alertDomain.Alert, error) {
	return f.alert, nil
}

type fakeEvTelemetryLookup struct {
	records []*alertDomain.TelemetryRecord
}

func (f *fakeEvTelemetryLookup) FindByAlertID(_ context.Context, _ string) ([]*alertDomain.TelemetryRecord, error) {
	return f.records, nil
}

type fakeEvAcousticLookup struct {
	summary *audioDomain.AcousticSummary
}

func (f *fakeEvAcousticLookup) AcousticSummary(_ context.Context, _ string) (*audioDomain.AcousticSummary, error) {
	return f.summary, nil
}

type fakeContactAuth struct {
	verified bool
}

func (f *fakeContactAuth) IsObserverVerified(_ context.Context, _, _ string) (bool, error) {
	return f.verified, nil
}

func newTestEvUC(alert *alertDomain.Alert, telemetry []*alertDomain.TelemetryRecord, verified bool) (domain.EvidenceUseCase, *fakeEvidenceRepo) {
	repo := newFakeEvidenceRepo()
	uc := NewEvidenceUseCase(
		&fakeEvAlertLookup{alert: alert},
		&fakeEvTelemetryLookup{records: telemetry},
		&fakeEvAcousticLookup{summary: &audioDomain.AcousticSummary{
			TotalChunks:        3,
			AvgStress:          0.5,
			PeakStress:         0.9,
			DistressAlerts:     1,
			EmotionalBreakdown: map[string]int{"neutral": 2, "fear": 1},
		}},
		repo,
		&fakeContactAuth{verified: verified},
	)
	return uc, repo
}

func baseAlert() *alertDomain.Alert {
	now := time.Now().UTC()
	return &alertDomain.Alert{
		ID: "alt_1", UserID: "usr_v", Type: alertDomain.AlertTypeSOS,
		Status: alertDomain.AlertStatusResolved, Latitude: 19.4, Longitude: -99.1,
		CreatedAt: now, ResolvedAt: &now,
	}
}

func TestGenerate_VictimCanAccess(t *testing.T) {
	uc, repo := newTestEvUC(baseAlert(), nil, false)

	report, err := uc.Generate(context.Background(), "alt_1", "usr_v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.SHA256 == "" {
		t.Fatal("expected sha256 hash")
	}
	if report.Acoustic.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", report.Acoustic.TotalChunks)
	}
	if len(repo.reports) != 1 {
		t.Errorf("expected report persisted")
	}
}

func TestGenerate_VerifiedObserverCanAccess(t *testing.T) {
	uc, _ := newTestEvUC(baseAlert(), nil, true)

	if _, err := uc.Generate(context.Background(), "alt_1", "usr_obs"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerate_UnverifiedObserverForbidden(t *testing.T) {
	uc, _ := newTestEvUC(baseAlert(), nil, false)

	_, err := uc.Generate(context.Background(), "alt_1", "usr_obs")
	if !errors.Is(err, ErrReportForbidden) {
		t.Fatalf("expected ErrReportForbidden, got %v", err)
	}
}

func TestGenerate_NotFound(t *testing.T) {
	uc, _ := newTestEvUC(nil, nil, false)

	_, err := uc.Generate(context.Background(), "alt_nope", "usr_v")
	if !errors.Is(err, ErrReportNotFound) {
		t.Fatalf("expected ErrReportNotFound, got %v", err)
	}
}

func TestGenerate_IncludesTelemetryStartEnd(t *testing.T) {
	telemetry := []*alertDomain.TelemetryRecord{
		{AlertID: "alt_1", Timestamp: 100, Latitude: 1, Longitude: 2},
		{AlertID: "alt_1", Timestamp: 200, Latitude: 3, Longitude: 4},
	}
	uc, _ := newTestEvUC(baseAlert(), telemetry, false)

	report, err := uc.Generate(context.Background(), "alt_1", "usr_v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TelemetryCount != 2 {
		t.Errorf("expected 2 telemetry records, got %d", report.TelemetryCount)
	}
	if report.TelemetryStart == nil || *report.TelemetryStart != 100 {
		t.Errorf("expected start 100, got %v", report.TelemetryStart)
	}
	if report.TelemetryEnd == nil || *report.TelemetryEnd != 200 {
		t.Errorf("expected end 200, got %v", report.TelemetryEnd)
	}
	if len(report.Path) != 2 {
		t.Errorf("expected 2 path points, got %d", len(report.Path))
	}
}

func TestGenerate_ImmutableReturn(t *testing.T) {
	uc, _ := newTestEvUC(baseAlert(), nil, false)

	r1, err := uc.Generate(context.Background(), "alt_1", "usr_v")
	if err != nil {
		t.Fatalf("first generate failed: %v", err)
	}
	r2, err := uc.Generate(context.Background(), "alt_1", "usr_v")
	if err != nil {
		t.Fatalf("second generate failed: %v", err)
	}
	if r1.SHA256 != r2.SHA256 {
		t.Errorf("immutable report hash changed: %s vs %s", r1.SHA256, r2.SHA256)
	}
}

func TestGenerate_HashIsStable(t *testing.T) {
	uc, repo := newTestEvUC(baseAlert(), nil, false)
	_, err := uc.Generate(context.Background(), "alt_1", "usr_v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report := repo.reports["alt_1"]
	if report == nil {
		t.Fatal("report not persisted")
	}

	// Recalcular el hash manualmente: el JSON del reporte sin el campo SHA256.
	clone := *report
	clone.SHA256 = ""
	sum, err := hashReport(&clone)
	if err != nil {
		t.Fatalf("rehash failed: %v", err)
	}
	if sum != report.SHA256 {
		t.Errorf("hash mismatch: %s vs %s", sum, report.SHA256)
	}
}

// TestGenerate_ConcurrentRaceReturnsExisting simula la carrera TOCTOU: otra
// solicitud insertó el expediente antes que esta; el Save falla con
// ErrReportAlreadyExists y el usecase debe devolver el guardado sin error.
func TestGenerate_ConcurrentRaceReturnsExisting(t *testing.T) {
	repo := newFakeEvidenceRepo()
	repo.failSaveWithDuplicate = true
	uc := NewEvidenceUseCase(
		&fakeEvAlertLookup{alert: baseAlert()},
		&fakeEvTelemetryLookup{records: nil},
		&fakeEvAcousticLookup{summary: &audioDomain.AcousticSummary{
			TotalChunks:        1,
			EmotionalBreakdown: map[string]int{"neutral": 1},
		}},
		repo,
		&fakeContactAuth{verified: false},
	)

	// Pre-insertar el expediente "ganador" de la carrera.
	winner := &domain.EvidenceReport{
		ID:        "alt_1",
		CreatedAt: time.Now().UTC(),
		SHA256:    "winner-hash",
	}
	repo.reports["alt_1"] = winner

	report, err := uc.Generate(context.Background(), "alt_1", "usr_v")
	if err != nil {
		t.Fatalf("race must not error, got %v", err)
	}
	if report == nil || report.SHA256 != "winner-hash" {
		t.Errorf("expected the winner report to be returned, got %+v", report)
	}
}
