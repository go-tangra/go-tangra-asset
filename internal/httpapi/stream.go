package httpapi

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-freya/freya/services/asset/internal/stream"
)

// RegisterStream mounts GET /api/asset/v1/stream: a per-signed-in-user SSE
// stream of asset realtime events (assigned/unassigned, warranty/license/
// insurance expiring, low stock) relayed from the tenant's platform event bus.
// Register calls this only when Deps.Hub is set, so a service wired without a
// hub leaves the route returning 501 not_implemented.
func (s *Server) RegisterStream(hub *stream.Hub) {
	instance, _ := os.Hostname()
	s.MustHandle("GET", "/api/asset/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		subj, err := subjects(r)
		if err != nil {
			failSvc(w, err)
			return
		}
		sub, err := hub.Subscribe(r.Context(), subj.TenantID, subj.UserID, r.URL.Query().Get("last_id"))
		if err != nil {
			if errors.Is(err, stream.ErrTooMany) {
				Fail(w, r, nil, ErrRateLimited)
				return
			}
			Fail(w, r, s.rt.Logger(), err)
			return
		}
		stream.ServeSSE(w, r, sub, instance, stream.Heartbeat, stream.MaxAge)
	})
}
