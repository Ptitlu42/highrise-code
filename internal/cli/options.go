package cli

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ptitlu42/highrise-code/internal/scan"
)

type Options struct {
	Mode           string
	Color_by       string
	Height_by      string
	Max_depth      int
	Ignore         []string
	Include_hidden bool
	No_gitignore   bool
	Max_files      int64
	Max_file_size  int64
	Workers        int
	Port           int
	No_open        bool
	Pick           bool
	Idle_timeout   time.Duration
	Out            string
	Stats          bool
	Format         string
	Strict         bool
	Verbose        bool
	Quiet          bool
	Help           bool
	Show_version   bool
	Root           string
	Notes          []string
}

type usage_error struct {
	message string
}

func (e *usage_error) Error() string {
	return e.message
}

func usage_errorf(format string, args ...any) error {
	return &usage_error{message: fmt.Sprintf(format, args...)}
}

func Is_usage_error(err error) bool {
	var u *usage_error
	return errors.As(err, &u)
}

type flag_spec struct {
	short     byte
	long      string
	has_value bool
	apply     func(o *Options, value string) error
}

func choice(name string, target func(o *Options) *string, allowed ...string) func(*Options, string) error {
	return func(o *Options, v string) error {
		for _, a := range allowed {
			if v == a {
				*target(o) = v
				return nil
			}
		}
		return usage_errorf("invalid value %q for --%s (expected %s)", v, name, strings.Join(allowed, ", "))
	}
}

func parse_int(name string, v string, min, max int) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, usage_errorf("invalid number %q for --%s", v, name)
	}
	if n < min || n > max {
		return 0, usage_errorf("value %d for --%s out of range [%d, %d]", n, name, min, max)
	}
	return n, nil
}

func flag_specs() []flag_spec {
	return []flag_spec{
		{'m', "mode", true, choice("mode", func(o *Options) *string { return &o.Mode }, "iso", "3d", "2d")},
		{'c', "color-by", true, choice("color-by", func(o *Options) *string { return &o.Color_by }, "language", "depth", "size")},
		{'H', "height-by", true, choice("height-by", func(o *Options) *string { return &o.Height_by }, "lines", "code", "size", "flat")},
		{'d', "max-depth", true, func(o *Options, v string) error {
			n, err := parse_int("max-depth", v, 1, 1<<30)
			if err != nil {
				return err
			}
			if n > scan.Hard_max_depth {
				o.Notes = append(o.Notes, fmt.Sprintf("--max-depth capped to %d", scan.Hard_max_depth))
				n = scan.Hard_max_depth
			}
			o.Max_depth = n
			return nil
		}},
		{'i', "ignore", true, func(o *Options, v string) error {
			if v == "" {
				return usage_errorf("empty pattern for --ignore")
			}
			o.Ignore = append(o.Ignore, v)
			return nil
		}},
		{0, "include-hidden", false, func(o *Options, _ string) error { o.Include_hidden = true; return nil }},
		{0, "no-gitignore", false, func(o *Options, _ string) error { o.No_gitignore = true; return nil }},
		{0, "max-files", true, func(o *Options, v string) error {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 1 {
				return usage_errorf("invalid value %q for --max-files", v)
			}
			o.Max_files = n
			return nil
		}},
		{0, "max-file-size", true, func(o *Options, v string) error {
			n, err := Parse_size(v)
			if err != nil {
				return usage_errorf("--max-file-size: %v", err)
			}
			o.Max_file_size = n
			return nil
		}},
		{0, "workers", true, func(o *Options, v string) error {
			n, err := parse_int("workers", v, 1, 1<<30)
			if err != nil {
				return err
			}
			if n > scan.Hard_max_workers {
				o.Notes = append(o.Notes, fmt.Sprintf("--workers capped to %d", scan.Hard_max_workers))
				n = scan.Hard_max_workers
			}
			o.Workers = n
			return nil
		}},
		{0, "port", true, func(o *Options, v string) error {
			n, err := parse_int("port", v, 0, 65535)
			if err != nil {
				return err
			}
			o.Port = n
			return nil
		}},
		{0, "no-open", false, func(o *Options, _ string) error { o.No_open = true; return nil }},
		{0, "pick", false, func(o *Options, _ string) error { o.Pick = true; return nil }},
		{0, "idle-timeout", true, func(o *Options, v string) error {
			d, err := time.ParseDuration(v)
			if err != nil || d < 0 {
				return usage_errorf("invalid duration %q for --idle-timeout", v)
			}
			o.Idle_timeout = d
			return nil
		}},
		{0, "out", true, func(o *Options, v string) error {
			if v == "" {
				return usage_errorf("empty path for --out")
			}
			o.Out = v
			return nil
		}},
		{0, "stats", false, func(o *Options, _ string) error { o.Stats = true; return nil }},
		{0, "format", true, choice("format", func(o *Options) *string { return &o.Format }, "text", "json")},
		{0, "strict", false, func(o *Options, _ string) error { o.Strict = true; return nil }},
		{'v', "verbose", false, func(o *Options, _ string) error { o.Verbose = true; return nil }},
		{'q', "quiet", false, func(o *Options, _ string) error { o.Quiet = true; return nil }},
		{'h', "help", false, func(o *Options, _ string) error { o.Help = true; return nil }},
		{'V', "version", false, func(o *Options, _ string) error { o.Show_version = true; return nil }},
	}
}

func default_options() Options {
	return Options{
		Mode:          "3d",
		Color_by:      "language",
		Height_by:     "lines",
		Max_depth:     scan.Hard_max_depth,
		Max_files:     scan.Default_max_files,
		Max_file_size: scan.Default_max_filesize,
		Workers:       min(runtime.NumCPU(), scan.Hard_max_workers),
		Format:        "text",
	}
}

func Parse_args(args []string) (Options, error) {
	opts := default_options()
	specs := flag_specs()
	by_long := map[string]*flag_spec{}
	by_short := map[byte]*flag_spec{}
	for i := range specs {
		by_long[specs[i].long] = &specs[i]
		if specs[i].short != 0 {
			by_short[specs[i].short] = &specs[i]
		}
	}
	var positional []string
	explicit_format := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			name, value, has_inline := strings.Cut(arg[2:], "=")
			spec, ok := by_long[name]
			if !ok {
				return opts, usage_errorf("unknown option --%s", name)
			}
			if spec.has_value {
				if !has_inline {
					if i+1 >= len(args) {
						return opts, usage_errorf("option --%s requires a value", name)
					}
					i++
					value = args[i]
				}
			} else if has_inline {
				return opts, usage_errorf("option --%s does not take a value", name)
			}
			if name == "format" {
				explicit_format = true
			}
			if err := spec.apply(&opts, value); err != nil {
				return opts, err
			}
		case len(arg) > 1 && arg[0] == '-':
			for j := 1; j < len(arg); j++ {
				spec, ok := by_short[arg[j]]
				if !ok {
					return opts, usage_errorf("unknown option -%c", arg[j])
				}
				if !spec.has_value {
					if err := spec.apply(&opts, ""); err != nil {
						return opts, err
					}
					continue
				}
				value := arg[j+1:]
				if value == "" {
					if i+1 >= len(args) {
						return opts, usage_errorf("option -%c requires a value", arg[j])
					}
					i++
					value = args[i]
				}
				if err := spec.apply(&opts, value); err != nil {
					return opts, err
				}
				break
			}
		default:
			positional = append(positional, arg)
		}
	}
	if opts.Help || opts.Show_version {
		return opts, nil
	}
	if opts.Verbose && opts.Quiet {
		return opts, usage_errorf("--verbose and --quiet are mutually exclusive")
	}
	if explicit_format && !opts.Stats {
		return opts, usage_errorf("--format requires --stats")
	}
	switch len(positional) {
	case 0:
		if !opts.Pick {
			return opts, usage_errorf("missing project root")
		}
	case 1:
		if opts.Pick {
			return opts, usage_errorf("--pick cannot be combined with a project root")
		}
		opts.Root = positional[0]
	default:
		return opts, usage_errorf("too many arguments: expected a single project root")
	}
	return opts, nil
}
