package services

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// buildPCM genera muestras PCM 16-bit mono de amplitud constante.
func buildPCM(samples int, amplitude float64) []byte {
	out := make([]byte, 0, samples*2)
	for i := 0; i < samples; i++ {
		v := int16(amplitude)
		out = append(out, byte(v), byte(v>>8))
	}
	return out
}

// buildWAV envuelve muestras PCM en un encabezado RIFF/WAVE.
func buildWAV(pcm []byte) []byte {
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+len(pcm)))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(header[22:24], 1) // mono
	binary.LittleEndian.PutUint32(header[24:28], 16000)
	binary.LittleEndian.PutUint32(header[28:32], 32000)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(len(pcm)))
	return append(header, pcm...)
}

// testServer monta un microservicio de IA simulado.
func testServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func TestAnalyze_MapsResponseAndThreshold(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/analyze" {
			t.Errorf("expected /analyze, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart/form-data, got %s", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("failed to parse multipart form: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("expected file field: %v", err)
		}
		_ = header
		_ = file
		_ = json.NewEncoder(w).Encode(aiServiceResponse{
			StressScore: 0.91,
			Emotion:     "miedo",
			Confidence:  0.91,
			IsDistress:  true,
		})
	})

	a := NewWav2Vec2Analyzer(server.URL, 5*time.Second, 0.85, 0.6)
	res, err := a.Analyze(context.Background(), "wav", buildWAV(buildPCM(16000, 30000)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StressScore != 0.91 {
		t.Errorf("expected stress_score 0.91, got %v", res.StressScore)
	}
	if !res.DistressDetected {
		t.Errorf("score 0.91 must be distress with threshold 0.85")
	}
	if res.PrimaryEmotion != "miedo" {
		t.Errorf("expected miedo, got %s", res.PrimaryEmotion)
	}
	if res.Confidence != 0.91 {
		t.Errorf("expected confidence 0.91, got %v", res.Confidence)
	}
}

func TestAnalyze_BelowThresholdIsNotDistress(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(aiServiceResponse{
			StressScore: 0.5,
			Emotion:     "neutral",
			Confidence:  0.5,
			IsDistress:  false,
		})
	})

	a := NewWav2Vec2Analyzer(server.URL, 5*time.Second, 0.85, 0.6)
	res, err := a.Analyze(context.Background(), "wav", buildWAV(buildPCM(16000, 2)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DistressDetected {
		t.Errorf("score 0.5 must not be distress, got %v", res.StressScore)
	}
}

func TestAnalyze_RawPCMIsSentAsWAV(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("failed to parse multipart form: %v", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("expected file field: %v", err)
			return
		}
		buf := make([]byte, 4)
		if _, err := file.Read(buf); err != nil {
			t.Errorf("failed to read payload: %v", err)
			return
		}
		if string(buf) != "RIFF" {
			t.Errorf("pcm must be wrapped in a wav header, got %q", buf)
		}
		_ = json.NewEncoder(w).Encode(aiServiceResponse{Emotion: "neutral"})
	})

	a := NewWav2Vec2Analyzer(server.URL, 5*time.Second, 0.85, 0.6)
	if _, err := a.Analyze(context.Background(), "pcm", buildPCM(8000, 5000)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnalyze_UnsupportedFormats(t *testing.T) {
	a := NewWav2Vec2Analyzer("http://127.0.0.1:1", time.Second, 0.85, 0.6)
	if _, err := a.Analyze(context.Background(), "opus", []byte{0x00, 0x01}); err == nil {
		t.Fatal("opus must be rejected")
	}
	if _, err := a.Analyze(context.Background(), "mp3", []byte{0x00, 0x01}); err == nil {
		t.Fatal("mp3 must be rejected")
	}
}

func TestAnalyze_EmptyAudio(t *testing.T) {
	a := NewWav2Vec2Analyzer("http://127.0.0.1:1", time.Second, 0.85, 0.6)
	if _, err := a.Analyze(context.Background(), "wav", nil); err == nil {
		t.Fatal("empty audio must be rejected")
	}
}

func TestAnalyze_ServerError(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	a := NewWav2Vec2Analyzer(server.URL, 5*time.Second, 0.85, 0.6)
	_, err := a.Analyze(context.Background(), "wav", buildWAV(buildPCM(100, 100)))
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestAnalyze_UnreachableService(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	a := NewWav2Vec2Analyzer(url, time.Second, 0.85, 0.6)
	_, err := a.Analyze(context.Background(), "wav", buildWAV(buildPCM(100, 100)))
	if err == nil {
		t.Fatal("expected error when service is unreachable")
	}
}

func TestAnalyze_InvalidJSON(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	})

	a := NewWav2Vec2Analyzer(server.URL, 5*time.Second, 0.85, 0.6)
	_, err := a.Analyze(context.Background(), "wav", buildWAV(buildPCM(100, 100)))
	if err == nil {
		t.Fatal("expected error on invalid json")
	}
}

func TestAnalyze_CancelledContext(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(aiServiceResponse{Emotion: "neutral"})
	})

	a := NewWav2Vec2Analyzer(server.URL, 5*time.Second, 0.85, 0.6)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Analyze(ctx, "wav", buildWAV(buildPCM(100, 100))); err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
