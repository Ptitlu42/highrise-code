package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ptitlu42/highrise-code/internal/scan"
	"github.com/ptitlu42/highrise-code/internal/stats"
)

func test_config(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"main.go":         "package main\n\n// hi\nfunc main() {}\n",
		"lib/util.py":     "# c\nx = 1\n",
		"lib/deep/a.js":   "var a = 1;\n",
		"lib/deep/b.js":   "var b = 2;\n",
		"docs/readme.md":  "# title\n",
		"docs/notes/x.md": "x\n",
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := scan.Run(context.Background(), scan.Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	return Config{
		Result:    res,
		Report:    stats.Compute(res),
		Mode:      "3d",
		Color_by:  "language",
		Height_by: "lines",
		Max_depth: scan.Hard_max_depth,
		Version:   "test",
	}
}

func new_test_app(t *testing.T) *App {
	t.Helper()
	app, err := New_app(test_config(t))
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func do_request(h http.Handler, method, target, host string, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.Host = host
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: cookie_name, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler_requires_session(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	for _, target := range []string{"/", "/api/summary", "/api/layout", "/js/main.js", "/css/style.css"} {
		if rec := do_request(h, http.MethodGet, target, host, ""); rec.Code != http.StatusForbidden {
			t.Fatalf("%s without cookie: %d", target, rec.Code)
		}
		if rec := do_request(h, http.MethodGet, target, host, "wrong"); rec.Code != http.StatusForbidden {
			t.Fatalf("%s with wrong cookie: %d", target, rec.Code)
		}
	}
}

func TestHandler_login_flow(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	rec := do_request(h, http.MethodGet, "/?t="+app.Token(), host, "")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("login: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != app.Token() || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("bad cookie: %+v", cookies)
	}
	if rec := do_request(h, http.MethodGet, "/?t=deadbeef", host, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("bad token: %d", rec.Code)
	}
	if rec := do_request(h, http.MethodPost, "/?t="+app.Token(), host, ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post login: %d", rec.Code)
	}
	if rec := do_request(h, http.MethodGet, "/", host, app.Token()); rec.Code != http.StatusOK {
		t.Fatalf("index: %d", rec.Code)
	}
}

func TestHandler_host_allow_list(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	for host, want := range map[string]int{
		"127.0.0.1:8123": http.StatusOK,
		"localhost:8123": http.StatusOK,
		"[::1]:8123":     http.StatusOK,
		"evil.test:8123": http.StatusForbidden,
		"127.0.0.1:9999": http.StatusForbidden,
		"127.0.0.1":      http.StatusForbidden,
		"":               http.StatusForbidden,
	} {
		rec := do_request(h, http.MethodGet, "/api/summary", host, app.Token())
		if rec.Code != want {
			t.Fatalf("host %q: got %d want %d", host, rec.Code, want)
		}
	}
}

func TestHandler_security_headers(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	rec := do_request(app.Handler(), http.MethodGet, "/", "127.0.0.1:8123", app.Token())
	for _, name := range []string{
		"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy",
		"Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy", "X-Frame-Options", "Cache-Control",
	} {
		if rec.Header().Get(name) == "" {
			t.Fatalf("missing header %s", name)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("weak csp: %s", csp)
	}
	if !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("csp without default-src none: %s", csp)
	}
}

func TestHandler_static_confinement(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	if rec := do_request(h, http.MethodGet, "/js/main.js", host, app.Token()); rec.Code != http.StatusOK ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("main.js: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := do_request(h, http.MethodGet, "/shaders/building.vert.glsl", host, app.Token()); rec.Code != http.StatusOK {
		t.Fatalf("shader: %d", rec.Code)
	}
	for _, target := range []string{"/js/../index.html", "/js/%2e%2e/index.html", "/js/nope.js", "/js/", "/css//style.css", "/js/main.js/x"} {
		if rec := do_request(h, http.MethodGet, target, host, app.Token()); rec.Code == http.StatusOK {
			t.Fatalf("%s should not be served", target)
		}
	}
	if rec := do_request(h, http.MethodPost, "/api/summary", host, app.Token()); rec.Code == http.StatusOK {
		t.Fatal("POST must not be served")
	}
}

func TestHandler_api_summary(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	rec := do_request(app.Handler(), http.MethodGet, "/api/summary", "127.0.0.1:8123", app.Token())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var s struct {
		Version string `json:"version"`
		Options struct {
			Standalone bool `json:"standalone"`
		} `json:"options"`
		Report struct {
			Totals struct {
				Files int64 `json:"files"`
			} `json:"totals"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.Version != "test" || s.Options.Standalone {
		t.Fatalf("unexpected summary: %+v", s)
	}
}

func TestHandler_api_layout_binary(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	rec := do_request(h, http.MethodGet, "/api/layout", host, app.Token())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.Bytes()
	if len(body) < 16 || string(body[:4]) != "HRL1" {
		t.Fatalf("bad magic: %q", body[:min(4, len(body))])
	}
	n := int(binary.LittleEndian.Uint32(body[4:8]))
	if len(body) != 49*n+16 {
		t.Fatalf("size %d for %d items", len(body), n)
	}
	shallow := do_request(h, http.MethodGet, "/api/layout?max_depth=1", host, app.Token())
	if shallow.Code != http.StatusOK {
		t.Fatalf("shallow status %d", shallow.Code)
	}
	shallow_n := int(binary.LittleEndian.Uint32(shallow.Body.Bytes()[4:8]))
	if shallow_n >= n {
		t.Fatalf("depth 1 should collapse items: %d >= %d", shallow_n, n)
	}
}

func TestHandler_api_node(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	rec := do_request(h, http.MethodGet, "/api/node?id=0", host, app.Token())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var d struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Kind != "dir" {
		t.Fatalf("root kind %q", d.Kind)
	}
	for _, target := range []string{"/api/node?id=-1", "/api/node?id=999999", "/api/node?id=abc", "/api/node"} {
		if rec := do_request(h, http.MethodGet, target, host, app.Token()); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", target, rec.Code)
		}
	}
}

func TestHandler_api_search(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	rec := do_request(h, http.MethodGet, "/api/search?q=util", host, app.Token())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var hits []search_hit
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Kind != "file" || !strings.HasSuffix(hits[0].Path, "util.py") {
		t.Fatalf("unexpected hits: %+v", hits)
	}
	long := "/api/search?q=" + strings.Repeat("a", 300)
	if rec := do_request(h, http.MethodGet, long, host, app.Token()); rec.Code != http.StatusBadRequest {
		t.Fatalf("long query: %d", rec.Code)
	}
}

func TestHandler_api_depth_clamped(t *testing.T) {
	app := new_test_app(t)
	app.port = 8123
	h := app.Handler()
	host := "127.0.0.1:8123"
	for _, depth := range []string{"-5", "0", "abc", "100000", "1"} {
		rec := do_request(h, http.MethodGet, "/api/layout?max_depth="+depth, host, app.Token())
		if rec.Code != http.StatusOK {
			t.Fatalf("depth %q: %d", depth, rec.Code)
		}
	}
}

func TestLayout_cache_bounded(t *testing.T) {
	app := new_test_app(t)
	for depth := 1; depth <= 12; depth++ {
		if _, err := app.store.layout_for(depth); err != nil {
			t.Fatal(err)
		}
	}
	app.store.mu.Lock()
	size := len(app.store.layouts)
	app.store.mu.Unlock()
	if size > max_cached_layouts {
		t.Fatalf("cache grew to %d", size)
	}
}

func TestWrite_standalone(t *testing.T) {
	var buf bytes.Buffer
	if err := Write_standalone(&buf, test_config(t)); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, want := range []string{"Content-Security-Policy", `type="importmap"`, "hr_summary", "hr_layout", "hr_names"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	for _, marker := range []string{"{{head}}", "{{boot}}"} {
		if strings.Contains(html, marker) {
			t.Fatalf("unreplaced %s", marker)
		}
	}
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Fatal("standalone page must not reference remote urls")
	}
	if !strings.Contains(html, `"standalone":true`) {
		t.Fatal("summary must flag standalone mode")
	}
}

func TestValid_url(t *testing.T) {
	good := []string{"http://127.0.0.1:8080/?t=abc", "http://127.0.0.1:1/"}
	bad := []string{"", "https://127.0.0.1:8080/", "http://localhost:8080/", "http://evil.test/", "http://u@127.0.0.1:80/", "http://127.0.0.1/", "file:///etc/passwd", "javascript:alert(1)", "-oProxyCommand=x"}
	for _, u := range good {
		if !Valid_url(u) {
			t.Fatalf("%q should be valid", u)
		}
	}
	for _, u := range bad {
		if Valid_url(u) {
			t.Fatalf("%q should be invalid", u)
		}
	}
}

func TestServe_end_to_end(t *testing.T) {
	app := new_test_app(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- app.Serve(ctx, Options{Port: 0, On_listen: func(u string) { urls <- u }})
	}()
	var url string
	select {
	case url = <-urls:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("unexpected url %q", url)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login status %d", resp.StatusCode)
	}
	port := app.port
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/summary", nil)
	req.AddCookie(resp.Cookies()[0])
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("summary status %d", resp.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop")
	}
	if conn, err := net.DialTimeout("tcp4", "127.0.0.1:"+strconv.Itoa(port), time.Second); err == nil {
		conn.Close()
		t.Fatal("port still open after shutdown")
	}
}

func TestServe_idle_timeout(t *testing.T) {
	app := new_test_app(t)
	done := make(chan error, 1)
	go func() {
		done <- app.Serve(context.Background(), Options{Port: 0, Idle_timeout: 1500 * time.Millisecond})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("idle timeout did not stop the server")
	}
}
