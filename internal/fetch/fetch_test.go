package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	in := `<html><head><style>body{}</style><script>var x=1;</script></head>
	<body><h1>Title</h1><p>Hello &amp; welcome</p></body></html>`
	out := HTMLToText(in)
	if strings.Contains(out, "var x") || strings.Contains(out, "body{}") {
		t.Errorf("script/style not stripped: %q", out)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "Hello & welcome") {
		t.Errorf("text/entities not preserved: %q", out)
	}
}

func TestFetchHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><p>fetched content</p></body></html>"))
	}))
	defer srv.Close()

	out, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "fetched content") {
		t.Errorf("got %q", out)
	}
}

func TestFetchNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL); err == nil {
		t.Error("expected error for 404")
	}
}
