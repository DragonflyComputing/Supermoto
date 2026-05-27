package supermoto

import (
	"log"
	"net/http"
	"time"
)

// Middleware holds a list of middleware to apply to a handler.
// Register middleware with Add(), then wrap your mux with Wrap().
//
//	mw := supermoto.NewMiddleware()
//	mw.Add(supermoto.Timer(nil))
//	mw.Add(auth)
//	http.ListenAndServe(":8080", mw.Wrap(mux))
type Middleware struct {
	stack []func(http.Handler) http.Handler
}

func NewMiddleware() *Middleware {
	return &Middleware{}
}

// Add registers a middleware function. Middleware executes in the order it is added.
func (m *Middleware) Add(mw func(http.Handler) http.Handler) {
	m.stack = append(m.stack, mw)
}

// Wrap applies all registered middleware to h and returns the result.
// Call this once when passing your mux to http.ListenAndServe.
func (m *Middleware) Wrap(h http.Handler) http.Handler {
	for i := len(m.stack) - 1; i >= 0; i-- {
		h = m.stack[i](h)
	}
	return h
}

// Timer returns middleware that logs how long each request takes.
// Pass nil to use the default standard library logger.
//
//	mw.Add(supermoto.Timer(nil))
func Timer(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Printf("%s %s took %v", r.Method, r.URL.Path, time.Since(start))
		})
	}
}
