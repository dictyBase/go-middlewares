package cache

import (
	"fmt"
	"net/http"
	"time"
)

// HTTPCache is a struct type for cache parameter
type HTTPCache struct {
	// MaxAge is value in seconds
	MaxAge int
	// Expires represents date and time in http format
	Expires string
}

// NeNeNewHTTPCache is a constructor for HTTPCache
func NewHTTPCache(day int, t time.Time) *HTTPCache {
	duration := time.Hour * 24 * 30 * day
	return &HTTPCache{
		MaxAge:  duration.Seconds(),
		Expires: t.Format(http.TimeFormat),
	}
}

// Hanlder is a net/http middleware for setting up
// max-age and Expires cache parameters
func (c *HTTPCache) Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", c.MaxAge))
		w.Header().Set("Expires", c.Expires)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}