package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ptitlu42/highrise-code/internal/scan"
	"github.com/ptitlu42/highrise-code/internal/server"
	"github.com/ptitlu42/highrise-code/internal/stats"
)

var Version = "dev"

const (
	exit_ok       = 0
	exit_fatal    = 1
	exit_usage    = 2
	exit_warnings = 3
	progress_tick = time.Second
	out_file_mode = 0o644
)

type Env struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Getenv    func(string) string
	Pick      func(context.Context) (string, error)
	Auto_pick bool
}

func Default_env() Env {
	return Env{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		Pick:      pick_folder,
		Auto_pick: launched_from_gui(os.Getenv),
	}
}

func Run(ctx context.Context, args []string, env Env) int {
	if len(args) == 0 && env.Auto_pick {
		args = []string{"--pick"}
	}
	opts, err := Parse_args(args)
	log := &logger{w: env.Stderr, level: level_normal, color: color_enabled(env.Stderr, env.Getenv)}
	if err != nil {
		log.errorf("%s", err)
		fmt.Fprintln(env.Stderr, "Try 'highrise-code --help' for more information.")
		return exit_usage
	}
	if opts.Help {
		fmt.Fprint(env.Stdout, usage_text)
		return exit_ok
	}
	if opts.Show_version {
		fmt.Fprintf(env.Stdout, "highrise-code %s\n", Version)
		return exit_ok
	}
	if opts.Quiet {
		log.level = level_quiet
	}
	if opts.Verbose {
		log.level = level_verbose
	}
	for _, n := range opts.Notes {
		log.debugf("%s", n)
	}
	if opts.Pick {
		if env.Pick == nil {
			log.errorf("%s", Err_no_picker)
			return exit_fatal
		}
		root, err := env.Pick(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.errorf("interrupted")
			} else {
				log.errorf("%s", Sanitize(err.Error()))
			}
			return exit_fatal
		}
		opts.Root = root
	}

	res, err := run_scan(ctx, opts, log)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.errorf("interrupted")
		} else {
			log.errorf("%s", Sanitize(err.Error()))
		}
		return exit_fatal
	}
	report_scan(log, res)
	rep := stats.Compute(res)

	code := exit_ok
	finish := func() int {
		if opts.Strict && res.Warnings_total > 0 && code == exit_ok {
			return exit_warnings
		}
		return code
	}

	standalone := opts.Out != "" || opts.Stats
	cfg := server.Config{
		Result:    res,
		Report:    rep,
		Mode:      opts.Mode,
		Color_by:  opts.Color_by,
		Height_by: opts.Height_by,
		Max_depth: opts.Max_depth,
		Version:   Version,
	}

	if opts.Out != "" {
		if err := write_out(opts.Out, env.Stdout, cfg); err != nil {
			log.errorf("--out: %s", Sanitize(err.Error()))
			return exit_fatal
		}
		if opts.Out != "-" {
			log.infof("wrote %s", Sanitize(opts.Out))
		}
	}
	if opts.Stats {
		if err := write_stats(env.Stdout, opts, rep, res, color_enabled(env.Stdout, env.Getenv)); err != nil {
			log.errorf("--stats: %s", Sanitize(err.Error()))
			return exit_fatal
		}
	}
	if standalone {
		return finish()
	}

	app, err := server.New_app(cfg)
	if err != nil {
		log.errorf("%s", err)
		return exit_fatal
	}
	serve_opts := server.Options{
		Port:         opts.Port,
		Idle_timeout: opts.Idle_timeout,
		On_listen: func(url string) {
			if !opts.Quiet || opts.No_open {
				fmt.Fprintf(env.Stderr, "highrise-code: serving at %s (Ctrl+C to stop)\n", url)
			}
			if !opts.No_open {
				if err := open_browser(url); err != nil {
					log.warnf("could not open the browser: %s", err)
				}
			}
		},
	}
	if err := app.Serve(ctx, serve_opts); err != nil {
		log.errorf("server: %s", err)
		return exit_fatal
	}
	if ctx.Err() == nil {
		log.infof("idle timeout reached, stopped")
	} else {
		log.debugf("stopped")
	}
	return finish()
}

func run_scan(ctx context.Context, opts Options, log *logger) (*scan.Result, error) {
	var counter atomic.Int64
	done := make(chan struct{})
	defer close(done)
	if log.level >= level_verbose {
		go func() {
			tick := time.NewTicker(progress_tick)
			defer tick.Stop()
			for {
				select {
				case <-done:
					return
				case <-tick.C:
					log.debugf("scanned %d files", counter.Load())
				}
			}
		}()
	}
	return scan.Run(ctx, scan.Options{
		Root:           opts.Root,
		Ignore:         opts.Ignore,
		Include_hidden: opts.Include_hidden,
		No_gitignore:   opts.No_gitignore,
		Max_files:      opts.Max_files,
		Max_file_size:  opts.Max_file_size,
		Max_depth:      scan.Hard_max_depth,
		Workers:        opts.Workers,
		Progress:       &counter,
	})
}

func report_scan(log *logger, res *scan.Result) {
	log.infof("scanned %s files in %s", group_int(res.Root.Files_total), res.Duration.Round(time.Millisecond))
	s := res.Skipped
	log.debugf("skipped: %d symlinks, %d special, %d binary, %d too large, %d ignored, %d hidden, %d unreadable, %d depth limit",
		s.Symlinks, s.Special, s.Binary, s.Too_large, s.Ignored, s.Hidden, s.Unreadable, s.Depth_limit)
	if res.Warnings_total == 0 {
		return
	}
	log.warnf("%s warnings during scan (use --verbose to list)", group_int(res.Warnings_total))
	for _, w := range res.Warnings {
		path := Sanitize(w.Path)
		if path == "" {
			path = "."
		}
		log.debugf("%s: %s", path, Sanitize(w.Message))
	}
	if res.Warnings_total > int64(len(res.Warnings)) {
		log.debugf("%s further warnings not retained", group_int(res.Warnings_total-int64(len(res.Warnings))))
	}
}

func write_stats(w io.Writer, opts Options, rep *stats.Report, res *scan.Result, color bool) error {
	if opts.Format == "json" {
		return stats.Write_json(w, rep, res)
	}
	return Write_text_stats(w, rep, color)
}

func write_out(path string, stdout io.Writer, cfg server.Config) error {
	if path == "-" {
		bw := bufio.NewWriterSize(stdout, 1<<16)
		if err := server.Write_standalone(bw, cfg); err != nil {
			return err
		}
		return bw.Flush()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".highrise-code-*")
	if err != nil {
		return err
	}
	tmp_name := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp_name)
	}
	bw := bufio.NewWriterSize(tmp, 1<<20)
	if err := server.Write_standalone(bw, cfg); err != nil {
		cleanup()
		return err
	}
	if err := bw.Flush(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Chmod(out_file_mode); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp_name)
		return err
	}
	if err := os.Rename(tmp_name, path); err != nil {
		_ = os.Remove(tmp_name)
		return err
	}
	return nil
}
