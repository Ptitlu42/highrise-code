package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ptitlu42/highrise-code/internal/layout"
	"github.com/ptitlu42/highrise-code/web"
)

const (
	cookie_name       = "hr_session"
	token_bytes       = 16
	shutdown_grace    = 5 * time.Second
	idle_poll         = time.Second
	max_header_bytes  = 16 << 10
	read_header_limit = 5 * time.Second
	read_limit        = 15 * time.Second
	write_limit       = 120 * time.Second
	idle_conn_limit   = 60 * time.Second
)

var content_types = map[string]string{
	".js":   "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".glsl": "text/plain; charset=utf-8",
	".html": "text/html; charset=utf-8",
}

type App struct {
	store    *store
	page     *page
	summary  []byte
	token    string
	port     int
	activity atomic.Int64
}

func New_app(cfg Config) (*App, error) {
	pg, err := served_page()
	if err != nil {
		return nil, err
	}
	sum, err := json.Marshal(build_summary(cfg, false))
	if err != nil {
		return nil, err
	}
	raw := make([]byte, token_bytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	a := &App{store: new_store(cfg), page: pg, summary: sum, token: hex.EncodeToString(raw)}
	a.touch()
	return a, nil
}

func (a *App) Token() string {
	return a.token
}

func (a *App) touch() {
	a.activity.Store(time.Now().UnixNano())
}

func (a *App) Idle_for() time.Duration {
	return time.Since(time.Unix(0, a.activity.Load()))
}

func (a *App) Url(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port) + "/?t=" + a.token
}

func (a *App) allowed_host(host string) bool {
	h, p, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	if a.port != 0 && p != strconv.Itoa(a.port) {
		return false
	}
	switch strings.ToLower(h) {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

func set_security_headers(h http.Header, csp string) {
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cache-Control", "no-store")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
}

func (a *App) authorized(r *http.Request) bool {
	c, err := r.Cookie(cookie_name)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(a.token)) == 1
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.handle_index)
	mux.HandleFunc("GET /api/summary", a.handle_summary)
	mux.HandleFunc("GET /api/layout", a.handle_layout)
	mux.HandleFunc("GET /api/names", a.handle_names)
	mux.HandleFunc("GET /api/node", a.handle_node)
	mux.HandleFunc("GET /api/search", a.handle_search)
	mux.HandleFunc("GET /css/", a.handle_static)
	mux.HandleFunc("GET /js/", a.handle_static)
	mux.HandleFunc("GET /shaders/", a.handle_static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set_security_headers(w.Header(), a.page.csp)
		if !a.allowed_host(r.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.touch()
		if r.URL.Path == "/" && r.URL.Query().Get("t") != "" {
			a.handle_login(w, r)
			return
		}
		if !a.authorized(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *App) handle_login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	given := r.URL.Query().Get("t")
	if subtle.ConstantTimeCompare([]byte(given), []byte(a.token)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookie_name,
		Value:    a.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handle_index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", content_types[".html"])
	_, _ = w.Write(a.page.bytes())
}

func (a *App) handle_summary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(a.summary)
}

func depth_param(r *http.Request) int {
	v, err := strconv.Atoi(r.URL.Query().Get("max_depth"))
	if err != nil {
		return 0
	}
	return v
}

func (a *App) handle_layout(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.layout_for(depth_param(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(c.encoded)))
	_, _ = w.Write(c.encoded)
}

func (a *App) handle_names(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.layout_for(depth_param(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(c.names_json)
}

func (a *App) handle_node(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.layout_for(depth_param(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id < 0 || id >= len(c.lay.Items) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	write_json(w, describe(c.lay, id))
}

func (a *App) handle_search(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.layout_for(depth_param(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query().Get("q")
	if len(q) > 256 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ids := c.lay.Search(q, max_search_results)
	hits := make([]search_hit, 0, len(ids))
	for _, id := range ids {
		kind := "file"
		switch c.lay.Items[id].Kind {
		case layout.Dir_kind:
			kind = "dir"
		case layout.Aggregate_kind:
			kind = "aggregate"
		}
		hits = append(hits, search_hit{Id: int(id), Path: c.lay.Path(int(id)), Kind: kind})
	}
	write_json(w, hits)
}

func write_json(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) handle_static(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if clean == "." || strings.HasPrefix(clean, "..") || clean != strings.TrimPrefix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(web.Files, clean)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct, ok := content_types[path.Ext(clean)]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(data)
}

type Options struct {
	Port         int
	Idle_timeout time.Duration
	On_listen    func(url string)
}

func (a *App) Serve(ctx context.Context, opts Options) error {
	ln, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(opts.Port))
	if err != nil {
		return err
	}
	a.port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{
		Handler:           a.Handler(),
		ReadHeaderTimeout: read_header_limit,
		ReadTimeout:       read_limit,
		WriteTimeout:      write_limit,
		IdleTimeout:       idle_conn_limit,
		MaxHeaderBytes:    max_header_bytes,
	}
	if opts.On_listen != nil {
		opts.On_listen(a.Url(a.port))
	}
	failed := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
		close(failed)
	}()
	idle := make(chan struct{})
	if opts.Idle_timeout > 0 {
		go func() {
			tick := time.NewTicker(idle_poll)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					if a.Idle_for() >= opts.Idle_timeout {
						close(idle)
						return
					}
				}
			}
		}()
	}
	var serve_err error
	select {
	case <-ctx.Done():
	case <-idle:
	case serve_err = <-failed:
	}
	shutdown_ctx, cancel := context.WithTimeout(context.Background(), shutdown_grace)
	defer cancel()
	if err := srv.Shutdown(shutdown_ctx); err != nil {
		_ = srv.Close()
	}
	return serve_err
}

func Valid_url(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" && (u.Hostname() == "127.0.0.1") && u.User == nil && u.Port() != ""
}
