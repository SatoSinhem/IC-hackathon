package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
	"github.com/ic-group/ic-tender/apps/api/internal/middleware"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB                pinger
	Queries           querier // *pgxpool.Pool; enables the tenders endpoints
	Auth              *auth.Service
	WebOrigin         string
	Logger            *slog.Logger
	Production        bool
	TrustForwardedFor bool
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", healthHandler)
	mux.HandleFunc("GET /api/v1/ready", readyHandler(d.DB))
	if d.Auth != nil {
		h := &authHandlers{svc: d.Auth, cookie: NewCookieConfig(d.Production), logger: d.Logger, trustForwarded: d.TrustForwardedFor, now: time.Now}
		h.register(mux)
		if d.Queries != nil {
			t := &tendersHandlers{db: d.Queries, auth: h, logger: d.Logger, now: time.Now}
			t.register(mux)
		}
	}
	return middleware.Chain(cors(d.WebOrigin, mux),
		middleware.RequestID(), middleware.AccessLog(d.Logger), middleware.Recover(d.Logger), sameOrigin(d.WebOrigin))
}
