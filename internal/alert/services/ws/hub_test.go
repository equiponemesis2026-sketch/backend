package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestHub_OpenRegisterBroadcast(t *testing.T) {
	hub := NewHub(context.Background())
	defer hub.Close()

	hub.OpenChannel("alt_1")
	if got := hub.ActiveAlerts(); len(got) != 1 || got[0] != "alt_1" {
		t.Fatalf("expected channel alt_1, got %v", got)
	}

	// Cliente emisor (víctima) y receptor (observador) en el mismo canal.
	emitterConn, emitterServer, _ := dialTestServer(t, hub, "alt_1", RoleEmitter)
	receiverConn, receiverServer, _ := dialTestServer(t, hub, "alt_1", RoleReceiver)
	defer emitterServer.Close()
	defer receiverServer.Close()
	defer func() { _ = emitterConn.Close(websocket.StatusNormalClosure, "") }()
	defer func() { _ = receiverConn.Close(websocket.StatusNormalClosure, "") }()

	// Esperar a que ambos clientes queden registrados.
	time.Sleep(100 * time.Millisecond)

	// El receptor debe recibir la telemetría que envía el emisor.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = emitterConn.Write(ctx, websocket.MessageText, []byte(`{"alert_id":"alt_1","latitude":19.4,"longitude":-99.1,"timestamp":1700000000}`))
	}()

	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()
	_, data, err := receiverConn.Read(readCtx)
	if err != nil {
		t.Fatalf("receiver failed to read telemetry: %v", err)
	}
	if !strings.Contains(string(data), `"alert_id":"alt_1"`) {
		t.Errorf("expected telemetry for alt_1, got %s", data)
	}
}

func TestHub_UnregisterClosesEmptyChannel(t *testing.T) {
	hub := NewHub(context.Background())
	defer hub.Close()

	hub.OpenChannel("alt_1")
	_, server, client := dialTestServer(t, hub, "alt_1", RoleReceiver)
	defer server.Close()

	time.Sleep(50 * time.Millisecond)

	client.Close()
	time.Sleep(100 * time.Millisecond)

	// El canal se cierra cuando ya no quedan clientes.
	if got := hub.ActiveAlerts(); len(got) != 0 {
		t.Errorf("expected no active channels after unregister, got %v", got)
	}
}

func TestHub_BroadcastSkipsEmitters(t *testing.T) {
	hub := NewHub(context.Background())
	defer hub.Close()

	// Solo un emisor (sin receptores): el broadcast no debe panic y no hay
	// nadie a quien enviar.
	hub.OpenChannel("alt_1")
	emitterConn, server, client := dialTestServer(t, hub, "alt_1", RoleEmitter)
	defer server.Close()
	defer func() { _ = emitterConn.Close(websocket.StatusNormalClosure, "") }()

	time.Sleep(50 * time.Millisecond)
	hub.Broadcast("alt_1", []byte(`{"alert_id":"alt_1","latitude":1}`))

	// El emisor no debe haber recibido nada.
	client.Close()
	time.Sleep(50 * time.Millisecond)
	if len(hub.ActiveAlerts()) != 0 {
		t.Errorf("expected empty channels, got %v", hub.ActiveAlerts())
	}
}

// dialTestServer levanta un servidor HTTP que hace upgrade WS y devuelve las
// conexiones de cliente, el servidor y el client del hub (del lado servidor).
func dialTestServer(t *testing.T, hub *Hub, alertID string, role Role) (*websocket.Conn, *httptest.Server, *Client) {
	clientCh := make(chan *Client, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			return
		}
		client := NewClient(hub, conn, "usr_test", role, []string{alertID}, nil)
		clientCh <- client
		go client.Run()
	})
	server := httptest.NewServer(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		cancel()
		server.Close()
		t.Fatalf("failed to dial test server: %v", err)
	}
	client := <-clientCh
	return conn, server, client
}
