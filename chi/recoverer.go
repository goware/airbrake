package chirecoverer

import (
	"net/http"

	"github.com/goware/airbrake"
	"github.com/pressly/chi"
	"golang.org/x/net/context"
)

func Recoverer(c *airbrake.Client) func(chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		return chi.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			if c != nil {
				defer c.HTTPPanicWithCtx(r, w, ctx)
			}
			next.ServeHTTPC(ctx, w, r)
		})
	}
}
