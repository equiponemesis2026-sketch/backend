package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	sendBufferSize = 32
)

// Client representa una conexión WebSocket única. Los clientes pueden ser
// emisores (smartwatch de la víctima) o receptores (dashboard del observador).
type Client struct {
	conn     *websocket.Conn
	hub      *Hub
	userID   string
	role     Role
	alertIDs map[string]struct{}
	store    alertDomain.TelemetryRepository
	send     chan []byte
	done     chan struct{}
	once     sync.Once
}

// NewClient crea un cliente asociado a un hub y sus canales de alerta.
// store es opcional; si se provee, la telemetría recibida se persiste en
// background para alimentar el historial forense (best-effort, sin bloquear).
func NewClient(hub *Hub, conn *websocket.Conn, userID string, role Role, alertIDs []string, store alertDomain.TelemetryRepository) *Client {
	c := &Client{
		conn:     conn,
		hub:      hub,
		userID:   userID,
		role:     role,
		alertIDs: make(map[string]struct{}, len(alertIDs)),
		store:    store,
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
	}
	for _, id := range alertIDs {
		c.alertIDs[id] = struct{}{}
	}
	return c
}

// Run inicia los pumps de lectura/escritura y el heartbeat. Bloquea hasta
// que la conexión finaliza.
func (c *Client) Run() {
	c.hub.Register(c)
	defer func() {
		c.hub.Unregister(c)
		c.Close()
	}()

	ctx := c.hub.ctx
	go c.writePump(ctx)
	go c.heartbeat(ctx)

	if c.role == RoleEmitter {
		c.readPump(ctx)
	} else {
		// Los receptores solo consumen; esperamos la cancelación del hub o
		// el cierre del cliente.
		<-c.done
	}
}

// Close cierra la conexión de forma idempotente.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close(websocket.StatusNormalClosure, "bye")
	})
}

// readPump procesa los paquetes de telemetría del emisor y los retransmite
// a los observadores del canal correspondiente.
func (c *Client) readPump(ctx context.Context) {
	defer c.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			slog.Debug("ws read error", "user_id", c.userID, "error", err)
			return
		}
		if typ != websocket.MessageText && typ != websocket.MessageBinary {
			continue
		}

		var packet alertDomain.TelemetryPacket
		if err := json.Unmarshal(data, &packet); err != nil {
			slog.Warn("ws invalid telemetry", "user_id", c.userID, "error", err)
			continue
		}

		if _, ok := c.alertIDs[packet.AlertID]; !ok {
			slog.Warn("ws emitter not subscribed to alert", "user_id", c.userID, "alert_id", packet.AlertID)
			continue
		}

		if packet.Timestamp == 0 {
			packet.Timestamp = time.Now().Unix()
		}

		if c.store != nil {
			c.persist(packet)
		}

		payload, err := json.Marshal(packet)
		if err != nil {
			continue
		}

		c.hub.Broadcast(packet.AlertID, payload)
	}
}

// persist guarda la telemetría en MongoDB de forma asíncrona y best-effort.
// Un fallo de persistencia jamás interrumpe la transmisión en vivo.
func (c *Client) persist(packet alertDomain.TelemetryPacket) {
	record := &alertDomain.TelemetryRecord{
		AlertID:      packet.AlertID,
		UserID:       c.userID,
		Latitude:     packet.Latitude,
		Longitude:    packet.Longitude,
		Speed:        packet.Speed,
		BatteryLevel: packet.BatteryLevel,
		Timestamp:    packet.Timestamp,
		ReceivedAt:   time.Now().UTC(),
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := c.store.Save(ctx, record); err != nil {
			slog.Warn("ws: failed to persist telemetry", "alert_id", packet.AlertID, "error", err)
		}
	}()
}

// writePump reenvía la telemetría a los receptores del cliente.
func (c *Client) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case msg := <-c.send:
			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				slog.Debug("ws write error", "user_id", c.userID, "error", err)
				return
			}
		}
	}
}

// heartbeat envía pings periódicos y cierra la conexión si no hay pong.
func (c *Client) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pongWait)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Debug("ws ping failed, closing", "user_id", c.userID, "error", err)
				c.Close()
				return
			}
		}
	}
}
