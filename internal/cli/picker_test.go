package cli

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func fake_look(available ...string) look_path {
	return func(name string) (string, error) {
		for _, a := range available {
			if a == name {
				return "/fake/" + name, nil
			}
		}
		return "", exec.ErrNotFound
	}
}

func TestFind_picker_per_platform(t *testing.T) {
	cases := []struct {
		goos      string
		available []string
		want      string
		fail      bool
	}{
		{"linux", []string{"zenity", "kdialog", "yad"}, "/fake/zenity", false},
		{"linux", []string{"kdialog", "yad"}, "/fake/kdialog", false},
		{"linux", []string{"yad"}, "/fake/yad", false},
		{"linux", nil, "", true},
		{"freebsd", []string{"zenity"}, "/fake/zenity", false},
		{"darwin", []string{"osascript"}, "/fake/osascript", false},
		{"darwin", nil, "", true},
		{"windows", []string{"powershell.exe"}, "/fake/powershell.exe", false},
		{"windows", []string{"pwsh.exe"}, "/fake/pwsh.exe", false},
		{"windows", nil, "", true},
	}
	for _, c := range cases {
		got, err := find_picker(c.goos, fake_look(c.available...), "/home/user")
		if c.fail {
			if !errors.Is(err, Err_no_picker) {
				t.Fatalf("%s %v: expected no picker, got %v", c.goos, c.available, err)
			}
			continue
		}
		if err != nil || got.path != c.want {
			t.Fatalf("%s %v: got %q err %v want %q", c.goos, c.available, got.path, err, c.want)
		}
		if len(got.args) == 0 {
			t.Fatalf("%s: no arguments", c.goos)
		}
	}
}

func TestFind_picker_windows_script_has_no_double_quotes(t *testing.T) {
	cmd, err := find_picker("windows", fake_look("powershell.exe"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range cmd.args {
		if strings.Contains(arg, "\"") {
			t.Fatalf("double quote in %q", arg)
		}
	}
}

func TestParse_picker_output(t *testing.T) {
	cases := map[string]string{
		"/tmp/x\n":           "/tmp/x",
		"/tmp/x\r\n":         "/tmp/x",
		"\ufeffC:\\work":     "C:\\work",
		"/tmp/a b/\nignored": "/tmp/a b/",
		"":                   "",
	}
	for input, want := range cases {
		if got := parse_picker_output([]byte(input)); got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
	}
}

func TestValidate_picked_dir(t *testing.T) {
	root := make_project(t)
	if got, err := validate_picked_dir(root); err != nil || got != root {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := validate_picked_dir(""); !errors.Is(err, Err_pick_cancelled) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := validate_picked_dir(root + "/main.go"); err == nil {
		t.Fatal("file accepted")
	}
	if _, err := validate_picked_dir(root + "/missing"); err == nil {
		t.Fatal("missing accepted")
	}
}

func TestRun_pick_uses_selected_folder(t *testing.T) {
	root := make_project(t)
	env, out, _ := test_env(t)
	env.Pick = func(context.Context) (string, error) { return root, nil }
	if code := Run(context.Background(), []string{"-q", "--stats", "--format", "json", "--pick"}, env); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "main.go") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestRun_auto_pick_without_arguments(t *testing.T) {
	root := make_project(t)
	env, _, errb := test_env(t)
	env.Auto_pick = true
	called := false
	env.Pick = func(context.Context) (string, error) {
		called = true
		return root, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := Run(ctx, nil, env)
	if !called {
		t.Fatalf("picker not called, exit %d stderr %q", code, errb.String())
	}
}

func TestRun_no_auto_pick_without_gui(t *testing.T) {
	env, _, errb := test_env(t)
	env.Pick = func(context.Context) (string, error) {
		t.Fatal("picker must not be called")
		return "", nil
	}
	if code := Run(context.Background(), nil, env); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errb.String(), "missing project root") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestRun_pick_cancelled(t *testing.T) {
	env, _, errb := test_env(t)
	env.Pick = func(context.Context) (string, error) { return "", Err_pick_cancelled }
	if code := Run(context.Background(), []string{"--pick"}, env); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errb.String(), "no folder selected") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestRun_pick_without_picker(t *testing.T) {
	env, _, _ := test_env(t)
	if code := Run(context.Background(), []string{"--pick"}, env); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestParse_args_pick(t *testing.T) {
	opts, err := Parse_args([]string{"--pick"})
	if err != nil || !opts.Pick || opts.Root != "" {
		t.Fatalf("opts %+v err %v", opts, err)
	}
	if _, err := Parse_args([]string{"--pick", "."}); !Is_usage_error(err) {
		t.Fatalf("expected usage error, got %v", err)
	}
}
