package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MichealJl/quark-nd-mcp/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestServer() *Server {
	return NewServer(&config.Config{Cookie: "test-cookie"})
}

func TestHTTPHealth(t *testing.T) {
	handler := newTestServer().HTTPHandler(HTTPOptions{})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Fatalf("health body = %q, want ok", body)
	}
}

func TestHTTPCORSPreflight(t *testing.T) {
	handler := newTestServer().HTTPHandler(HTTPOptions{Path: "/mcp"})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Mcp-Session-Id")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q", got)
	}
}

func TestHTTPBearerAuth(t *testing.T) {
	handler := newTestServer().HTTPHandler(HTTPOptions{Token: "secret", Path: "/mcp"})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	resp, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("unauthenticated request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	health, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	health.Body.Close()
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health should skip auth, got %d", health.StatusCode)
	}
}

func TestStreamableHTTPListTools(t *testing.T) {
	srv := newTestServer()
	handler := srv.HTTPHandler(HTTPOptions{Path: "/mcp"})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatal("expected registered tools, got none")
	}

	found := false
	for _, tool := range result.Tools {
		if tool.Name == "list_files" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list_files tool not found in %d tools", len(result.Tools))
	}
}
