package domain

import "context"

// VocalTensionResult es el resultado estructurado del análisis de tensión
// vocal de un fragmento de audio (proxy Wav2Vec2 / SER).
type VocalTensionResult struct {
	ChunkIndex       int       `json:"chunk_index"`
	StressScore      float64   `json:"stress_score"`
	DistressDetected bool      `json:"distress_detected"`
	Confidence       float64   `json:"confidence"`
	Embeddings       []float64 `json:"embeddings"`
	PrimaryEmotion   string    `json:"primary_emotion"`
}

// VocalTensionAnalyzer clasifica el nivel de estrés/tensión vocal de un
// fragmento de audio y extrae embeddings acústicos. La implementación local
// es un proxy determinista basado en energía RMS; puede sustituirse por una
// inferencia real de Wav2Vec2 (ONNX o servicio externo) manteniendo la misma
// interfaz.
type VocalTensionAnalyzer interface {
	Analyze(ctx context.Context, format string, audio []byte) (VocalTensionResult, error)
}
