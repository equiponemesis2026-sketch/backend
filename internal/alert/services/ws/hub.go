// Package ws implementa el gestor de conexiones WebSocket para la
// transmisión en vivo de telemetría de alertas (hub de canales).
package ws

import (
	"context"
	"sync"
)

// Role indica si un cliente emite telemetría (smartwatch/víctima) o la
// consume (dashboard/observador).
type Role int

const (
	RoleReceiver Role = iota
	RoleEmitter
)

// Channel agrupa los clientes suscritos a la telemetría de una alerta.
type Channel struct {
	alertID string
	clients map[*Client]struct{}
}

// Hub gestiona los canales activos de telemetría de forma concurrente.
type Hub struct {
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	channels map[string]*Channel
}

// NewHub crea un hub de telemetría.
func NewHub(ctx context.Context) *Hub {
	ctx, cancel := context.WithCancel(ctx)
	return &Hub{
		ctx:      ctx,
		cancel:   cancel,
		channels: make(map[string]*Channel),
	}
}

// OpenChannel crea (o recupera) el canal de una alerta.
func (h *Hub) OpenChannel(alertID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.channels[alertID]; !ok {
		h.channels[alertID] = &Channel{
			alertID: alertID,
			clients: make(map[*Client]struct{}),
		}
	}
}

// Register suscribe un cliente a sus canales.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for alertID := range c.alertIDs {
		ch, ok := h.channels[alertID]
		if !ok {
			ch = &Channel{alertID: alertID, clients: make(map[*Client]struct{})}
			h.channels[alertID] = ch
		}
		ch.clients[c] = struct{}{}
	}
}

// Unregister retira un cliente de todos sus canales y cierra el canal
// si quedó vacío.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for alertID := range c.alertIDs {
		ch, ok := h.channels[alertID]
		if !ok {
			continue
		}
		delete(ch.clients, c)
		if len(ch.clients) == 0 {
			delete(h.channels, alertID)
		}
	}
}

// Broadcast envía la telemetría a todos los clientes suscritos al canal.
// Los clientes lentos (buffer lleno) se cierran para no bloquear el flujo.
func (h *Hub) Broadcast(alertID string, payload []byte) {
	h.mu.RLock()
	ch, ok := h.channels[alertID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	for c := range ch.clients {
		if c.role != RoleReceiver {
			continue
		}
		select {
		case c.send <- payload:
		default:
			go c.Close()
		}
	}
	h.mu.RUnlock()
}

// Close cancela el contexto del hub y cierra todos los canales/clientes.
func (h *Hub) Close() {
	h.cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.channels {
		for c := range ch.clients {
			c.Close()
		}
	}
	h.channels = make(map[string]*Channel)
}

// ActiveAlerts devuelve los identificadores de canales actualmente abiertos.
func (h *Hub) ActiveAlerts() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.channels))
	for id := range h.channels {
		ids = append(ids, id)
	}
	return ids
}
