// Package ws expone el endpoint WebSocket de telemetría de alertas.
package ws

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"

	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
	wsHub "github.com/nemesis-project/api-nemesis/internal/alert/services/ws"
	contactDomain "github.com/nemesis-project/api-nemesis/internal/contact/domain"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
)

var errUnauthorized = errors.New("unauthorized")

// Deps agrupa las dependencias que necesita el handler WebSocket.
type Deps struct {
	Hub         *wsHub.Hub
	AlertRepo   alertDomain.AlertRepository
	ContactRepo contactDomain.ContactRepository
	AuthSecret  []byte
}

// Handler expone el upgrade de WebSocket para el streaming de telemetría.
type Handler struct {
	deps Deps
}

// NewHandler crea el handler de WebSocket.
func NewHandler(deps Deps) *Handler {
	return &Handler{deps: deps}
}

// Stream maneja GET /ws/v1/alerts/stream. Autentica el token JWT vía query
// param `token` o header `Authorization` y determina el rol del cliente.
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authenticate(r)
	if err != nil {
		http.Error(w, `{"status":"error","message":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	role, alertIDs, err := h.resolveRole(r.Context(), userID)
	if err != nil {
		slog.Error("ws: failed to resolve role", "user_id", userID, "error", err)
		http.Error(w, `{"status":"error","message":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Si el usuario no participa en ninguna alerta activa, no hay nada que transmitir.
	if len(alertIDs) == 0 {
		http.Error(w, `{"status":"error","message":"no active alerts for this user"}`, http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Debug("ws: accept failed", "user_id", userID, "error", err)
		return
	}

	client := wsHub.NewClient(h.deps.Hub, conn, userID, role, alertIDs)
	slog.Info("ws: client connected", "user_id", userID, "role", role, "alerts", len(alertIDs))
	client.Run()
}

// authenticate extrae y valida el user_id del JWT (query `token` o header).
func (h *Handler) authenticate(r *http.Request) (string, error) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenStr = parts[1]
			}
		}
	}
	if tokenStr == "" {
		return "", errUnauthorized
	}

	userID, err := middleware.ParseUserID(tokenStr, h.deps.AuthSecret)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// resolveRole determina si el usuario es emisor (víctima con alerta activa)
// o receptor (observador vinculado) y los canales a los que se suscribe.
func (h *Handler) resolveRole(ctx context.Context, userID string) (wsHub.Role, []string, error) {
	// Emisor: alerta activa propia.
	own, err := h.deps.AlertRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		return wsHub.RoleReceiver, nil, err
	}
	if own != nil {
		return wsHub.RoleEmitter, []string{own.ID}, nil
	}

	// Receptor: observador vinculado a víctimas con alerta activa.
	contacts, err := h.deps.ContactRepo.FindAllByLinkedUserID(ctx, userID)
	if err != nil {
		return wsHub.RoleReceiver, nil, err
	}
	victimIDs := make([]string, 0, len(contacts))
	for _, c := range contacts {
		victimIDs = append(victimIDs, c.UserID)
	}
	if len(victimIDs) == 0 {
		return wsHub.RoleReceiver, nil, nil
	}

	alerts, err := h.deps.AlertRepo.FindActiveByUserIDs(ctx, victimIDs)
	if err != nil {
		return wsHub.RoleReceiver, nil, err
	}
	alertIDs := make([]string, 0, len(alerts))
	for _, a := range alerts {
		alertIDs = append(alertIDs, a.ID)
	}
	return wsHub.RoleReceiver, alertIDs, nil
}
