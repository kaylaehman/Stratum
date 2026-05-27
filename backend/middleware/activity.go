package middleware

import (
	"context"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/kaylaehman/stratum/backend/activity"
)

// Activity wraps mutating routes and writes one append-only audit row per
// request. The write is performed in a deferred closure so it runs even when
// the handler panics — the row records result=error and the panic is re-raised
// for the outer Recoverer to convert to a 500 (foundation design §5.2/§5.4).
//
// Handlers enrich the audit row via activity.FromContext (set Action, Target,
// Detail). Streaming handlers set Entry.Suppressed and call activity.Append
// directly to avoid a double entry.
func Activity(store *activity.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			entry := &activity.Entry{Action: r.Method + " " + r.URL.Path}
			ctx := activity.NewContext(r.Context(), entry)

			defer func() {
				rec := recover()

				if !entry.Suppressed {
					switch {
					case rec != nil, ww.Status() >= 400:
						entry.Result = activity.ResultError
					default:
						entry.Result = activity.ResultSuccess
					}
					if entry.UserID == nil {
						if u, ok := UserFromContext(ctx); ok {
							id := u.ID
							entry.UserID = &id
						}
					}
					// Detach from request cancellation so the audit row is
					// written even if the client disconnected or the handler
					// panicked mid-flight; preserve context values.
					_ = store.Append(context.WithoutCancel(ctx), *entry)
				}

				if rec != nil {
					panic(rec) // let the outer Recoverer handle it
				}
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
