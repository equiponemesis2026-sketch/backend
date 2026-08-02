package services

import (
	"context"
	"encoding/binary"
	"math"
	"testing"
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

func TestAnalyze_SilenceIsNeutral(t *testing.T) {
	a := NewWav2Vec2Analyzer(0.85, 0.6)
	// Silencio casi total: amplitud muy baja.
	pcm := buildPCM(16000, 2)
	wav := buildWAV(pcm)

	res, err := a.Analyze(context.Background(), "wav", wav)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DistressDetected {
		t.Errorf("silence must not be distress, got score=%v", res.StressScore)
	}
	if res.PrimaryEmotion != "neutral" {
		t.Errorf("expected neutral, got %s", res.PrimaryEmotion)
	}
	if len(res.Embeddings) != 8 {
		t.Errorf("expected 8 embeddings, got %d", len(res.Embeddings))
	}
}

func TestAnalyze_HighAmplitudeIsDistress(t *testing.T) {
	a := NewWav2Vec2Analyzer(0.85, 0.6)
	// Amplitud casi máxima: tensión vocal alta.
	pcm := buildPCM(32000, 30000)
	wav := buildWAV(pcm)

	res, err := a.Analyze(context.Background(), "wav", wav)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.DistressDetected {
		t.Errorf("high amplitude must be distress, got score=%v", res.StressScore)
	}
	if res.StressScore <= 0.85 {
		t.Errorf("expected stress_score above threshold, got %v", res.StressScore)
	}
}

func TestAnalyze_RawPCM(t *testing.T) {
	a := NewWav2Vec2Analyzer(0.85, 0.6)
	pcm := buildPCM(8000, 5000)

	res, err := a.Analyze(context.Background(), "pcm", pcm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PrimaryEmotion == "" {
		t.Error("expected a primary emotion")
	}
}

func TestAnalyze_OpusRejected(t *testing.T) {
	a := NewWav2Vec2Analyzer(0.85, 0.6)
	_, err := a.Analyze(context.Background(), "opus", []byte{0x00, 0x01})
	if err == nil {
		t.Fatal("opus must be rejected")
	}
}

func TestAnalyze_InvalidWAV(t *testing.T) {
	a := NewWav2Vec2Analyzer(0.85, 0.6)
	_, err := a.Analyze(context.Background(), "wav", []byte("not a wav at all"))
	if err == nil {
		t.Fatal("invalid wav must be rejected")
	}
}

func TestRound4(t *testing.T) {
	if got := round4(math.Pi); got != 3.1416 {
		t.Errorf("expected 3.1416, got %v", got)
	}
}
