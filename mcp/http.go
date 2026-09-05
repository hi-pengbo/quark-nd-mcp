package mcp

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultHTTPAddr = "127.0.0.1:8080"
	defaultHTTPPath = "/mcp"
)

// HTTPOptions configures the Streamable HTTP transport.
type HTTPOptions struct {
	// Addr is the listen address, e.g. "127.0.0.1:8080".
	Addr string
	// Path is the MCP endpoint path. Defaults to "/mcp".
	Path string
	// Token, if set, requires Authorization: Bearer <token> on MCP requests.
	Token string
}

func (o HTTPOptions) withDefaults() HTTPOptions {
	if o.Addr == "" {
		o.Addr = defaultHTTPAddr
	}
	if o.Path == "" {
		o.Path = defaultHTTPPath
	}
	if !strings.HasPrefix(o.Path, "/") {
		o.Path = "/" + o.Path
	}
	return o
}

// HTTPHandler returns an http.Handler serving Streamable HTTP MCP sessions
// at opts.Path, plus GET /health.
func (s *Server) HTTPHandler(opts HTTPOptions) http.Handler {
	opts = opts.withDefaults()

	streamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.server
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 30 * time.Minute,
	})

	mux := http.NewServeMux()
	mux.Handle(opts.Path, streamHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return withCORS(withBearerAuth(opts.Token, mux))
}

// RunHTTP starts the MCP server using the Streamable HTTP transport.
func (s *Server) RunHTTP(ctx context.Context, opts HTTPOptions) error {
	opts = opts.withDefaults()

	httpServer := &http.Server{
		Addr:              opts.Addr,
		Handler:           s.HTTPHandler(opts),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown: %v", err)
		}
	}()

	log.Printf("MCP Streamable HTTP listening on http://%s%s", opts.Addr, opts.Path)
	if opts.Token != "" {
		log.Printf("Bearer token authentication enabled")
	}

	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func withBearerAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		if got != "Bearer "+token {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, Mcp-Session-Id, Mcp-Protocol-Version, Last-Event-ID")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
