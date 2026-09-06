package daemon

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
)

func (d *daemon) httpServer() {
	mux := http.NewServeMux()
	mux.Handle("/clients", http.HandlerFunc(d.httpRouteClients))
	http.Serve(d.httpListener, mux)
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
