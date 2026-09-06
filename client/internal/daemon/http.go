package daemon

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func (d *daemon) httpServer() {
	mux := http.NewServeMux()

	mux.Handle("GET /clients", http.HandlerFunc(d.httpRouteClients))
	mux.Handle("GET /clients/{id}/events", http.HandlerFunc(d.httpRouteClientEvents))

	http.Serve(d.httpListener, withCORS(mux))
}

func (d *daemon) httpRouteClients(w http.ResponseWriter, r *http.Request) {
	type clientInfo struct {
		ID string `json:"id"`
	}

	type response struct {
		Clients []clientInfo `json:"clients"`
	}

	d.mu.Lock()
	clients := make([]clientInfo, 0, len(d.clients))
	for _, c := range d.clients {
		clients = append(clients, clientInfo{ID: hex.EncodeToString(c.id[:])})
	}
	d.mu.Unlock()

	resp := response{
		Clients: clients,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (d *daemon) httpRouteClientEvents(w http.ResponseWriter, r *http.Request) {
	id, err := hex.DecodeString(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}
	var key [16]byte
	copy(key[:], id)
	var ch chan string

	d.mu.Lock()
	client, ok := d.clients[key]
	if ok {
		ch = client.subscribe()
		defer client.unsubscribe(ch)
	}
	d.mu.Unlock()

	if ch == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	/* Upgrade to a websocket */
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade to websocket", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
		case <-closed:
			return
		}
	}
}
