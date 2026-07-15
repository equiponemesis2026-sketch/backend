package domain

import "context"

type Telemetry struct {
	ID        string   `json:"id" bson:"_id,omitempty"`
	UserID    string   `json:"user_id" bson:"user_id"`
	BPM       int      `json:"bpm" bson:"bpm"`
	HRV       float64  `json:"hrv" bson:"hrv"` // RMSSD
	GPS       GPSData  `json:"gps" bson:"gps"`
	Timestamp int64    `json:"timestamp" bson:"timestamp"`
}

type GPSData struct {
	Latitude  float64 `json:"latitude" bson:"latitude"`
	Longitude float64 `json:"longitude" bson:"longitude"`
}

type TelemetryRepository interface {
	Store(ctx context.Context, telemetry *Telemetry) error
	GetLatestByUserID(ctx context.Context, userID string) (*Telemetry, error)
}

type TelemetryUseCase interface {
	Ingest(ctx context.Context, telemetry *Telemetry) error
}
