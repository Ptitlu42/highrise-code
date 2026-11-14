package server

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/ptitlu42/highrise-code/internal/layout"
	"github.com/ptitlu42/highrise-code/web"
)

const (
	head_marker = "{{head}}"
	boot_marker = "{{boot}}"
	boot_module = "import \"hr/main\";"
)

var shader_names = []string{"building.vert", "building.frag", "pick.frag"}

type page struct {
	before string
	middle string
	after  string
	head   string
	boot   string
	csp    string
}

func csp_hash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func read_asset(name string) ([]byte, error) {
	return fs.ReadFile(web.Files, name)
}

func module_names() ([]string, error) {
	entries, err := fs.ReadDir(web.Files, "js")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".js") {
			names = append(names, strings.TrimSuffix(n, ".js"))
		}
	}
	return names, nil
}

func split_template() (before, middle, after string, err error) {
	raw, err := read_asset("index.html")
	if err != nil {
		return "", "", "", err
	}
	text := string(raw)
	h := strings.Index(text, head_marker)
	b := strings.Index(text, boot_marker)
	if h < 0 || b < 0 || b < h {
		return "", "", "", fmt.Errorf("index.html: template markers missing")
	}
	return text[:h], text[h+len(head_marker) : b], text[b+len(boot_marker):], nil
}

func import_map_json(resolve func(name string) (string, error)) (string, error) {
	names, err := module_names()
	if err != nil {
		return "", err
	}
	imports := map[string]string{}
	for _, n := range names {
		url, err := resolve(n)
		if err != nil {
			return "", err
		}
		imports["hr/"+n] = url
	}
	b, err := json.Marshal(map[string]map[string]string{"imports": imports})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func served_page() (*page, error) {
	before, middle, after, err := split_template()
	if err != nil {
		return nil, err
	}
	im, err := import_map_json(func(n string) (string, error) { return "/js/" + n + ".js", nil })
	if err != nil {
		return nil, err
	}
	head := "<link rel=\"stylesheet\" href=\"/css/style.css\">\n<script type=\"importmap\">" + im + "</script>\n"
	boot := "<script type=\"module\" src=\"/js/main.js\"></script>\n"
	csp := strings.Join([]string{
		"default-src 'none'",
		"script-src 'self' " + csp_hash(im),
		"style-src 'self'",
		"img-src 'self' data:",
		"connect-src 'self'",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors 'none'",
	}, "; ")
	return &page{before: before, middle: middle, after: after, head: head, boot: boot, csp: csp}, nil
}

func (p *page) bytes() []byte {
	return []byte(p.before + p.head + p.middle + p.boot + p.after)
}

type base64_gzip struct {
	enc io.WriteCloser
	gz  *gzip.Writer
}

func new_base64_gzip(w io.Writer) *base64_gzip {
	enc := base64.NewEncoder(base64.StdEncoding, w)
	gz, _ := gzip.NewWriterLevel(enc, gzip.BestSpeed)
	return &base64_gzip{enc: enc, gz: gz}
}

func (b *base64_gzip) Write(p []byte) (int, error) {
	return b.gz.Write(p)
}

func (b *base64_gzip) Close() error {
	if err := b.gz.Close(); err != nil {
		return err
	}
	return b.enc.Close()
}

func Write_standalone(w io.Writer, cfg Config) error {
	before, middle, after, err := split_template()
	if err != nil {
		return err
	}
	im, err := import_map_json(func(n string) (string, error) {
		src, err := read_asset("js/" + n + ".js")
		if err != nil {
			return "", err
		}
		return "data:text/javascript;base64," + base64.StdEncoding.EncodeToString(src), nil
	})
	if err != nil {
		return err
	}
	css, err := read_asset("css/style.css")
	if err != nil {
		return err
	}
	style := string(css)
	csp := strings.Join([]string{
		"default-src 'none'",
		"script-src " + csp_hash(im) + " " + csp_hash(boot_module) + " data:",
		"style-src " + csp_hash(style),
		"img-src data:",
		"base-uri 'none'",
		"form-action 'none'",
	}, "; ")

	lay := layout.Compute(cfg.Result, cfg.Max_depth)
	sum, err := json.Marshal(build_summary(cfg, true))
	if err != nil {
		return err
	}

	if _, err := io.WriteString(w, before); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "<meta http-equiv=\"Content-Security-Policy\" content=\"%s\">\n<style>%s</style>\n<script type=\"importmap\">%s</script>\n", csp, style, im); err != nil {
		return err
	}
	if _, err := io.WriteString(w, middle); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "<script type=\"application/json\" id=\"hr_summary\">%s</script>\n", sum); err != nil {
		return err
	}
	for _, n := range shader_names {
		src, err := read_asset("shaders/" + n + ".glsl")
		if err != nil {
			return err
		}
		id := "hr_shader_" + strings.ReplaceAll(n, ".", "_")
		if _, err := fmt.Fprintf(w, "<script type=\"text/plain\" id=\"%s\">%s</script>\n", id, src); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "<script type=\"text/plain\" id=\"hr_layout\">"); err != nil {
		return err
	}
	bg := new_base64_gzip(w)
	if err := lay.Encode(bg); err != nil {
		return err
	}
	if err := bg.Close(); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "</script>\n<script type=\"text/plain\" id=\"hr_names\">"); err != nil {
		return err
	}
	bg = new_base64_gzip(w)
	all_names := make([]string, len(lay.Items))
	for i := range lay.Items {
		all_names[i] = lay.Name(i)
	}
	if err := json.NewEncoder(bg).Encode(all_names); err != nil {
		return err
	}
	if err := bg.Close(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "</script>\n<script type=\"module\">%s</script>\n", boot_module); err != nil {
		return err
	}
	_, err = io.WriteString(w, after)
	return err
}
