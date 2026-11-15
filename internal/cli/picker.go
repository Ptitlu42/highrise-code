package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const picker_title = "highrise-code: choose the project folder"

const windows_picker_script = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; " +
	"Add-Type -AssemblyName System.Windows.Forms; " +
	"$owner = New-Object System.Windows.Forms.Form; " +
	"$owner.TopMost = $true; " +
	"$dialog = New-Object System.Windows.Forms.FolderBrowserDialog; " +
	"$dialog.Description = '" + picker_title + "'; " +
	"$dialog.ShowNewFolderButton = $false; " +
	"if ($dialog.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($dialog.SelectedPath) }"

const macos_picker_script = "POSIX path of (choose folder with prompt \"" + picker_title + "\")"

var (
	Err_pick_cancelled = errors.New("no folder selected")
	Err_no_picker      = errors.New("no folder picker available (install zenity or kdialog, or pass the folder as an argument)")
)

type picker_command struct {
	path string
	args []string
}

type look_path func(string) (string, error)

func find_picker(goos string, look look_path, home string) (picker_command, error) {
	switch goos {
	case "windows":
		for _, name := range []string{"powershell.exe", "pwsh.exe"} {
			if path, err := look(name); err == nil {
				return picker_command{path, []string{"-NoProfile", "-NonInteractive", "-STA", "-Command", windows_picker_script}}, nil
			}
		}
	case "darwin":
		if path, err := look("osascript"); err == nil {
			return picker_command{path, []string{"-e", macos_picker_script}}, nil
		}
	default:
		if path, err := look("zenity"); err == nil {
			return picker_command{path, []string{"--file-selection", "--directory", "--title=" + picker_title}}, nil
		}
		if path, err := look("kdialog"); err == nil {
			start := home
			if start == "" {
				start = "."
			}
			return picker_command{path, []string{"--title", picker_title, "--getexistingdirectory", start}}, nil
		}
		if path, err := look("yad"); err == nil {
			return picker_command{path, []string{"--file", "--directory", "--title=" + picker_title}}, nil
		}
	}
	return picker_command{}, Err_no_picker
}

func parse_picker_output(raw []byte) string {
	text := strings.TrimPrefix(string(raw), "\ufeff")
	line, _, _ := strings.Cut(text, "\n")
	return strings.TrimRight(line, "\r")
}

func validate_picked_dir(selected string) (string, error) {
	if selected == "" {
		return "", Err_pick_cancelled
	}
	abs, err := filepath.Abs(selected)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", Sanitize(abs))
	}
	return abs, nil
}

func run_picker(ctx context.Context, cmd picker_command) (string, error) {
	var stdout, stderr bytes.Buffer
	proc := exec.CommandContext(ctx, cmd.path, cmd.args...)
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	err := proc.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	selected := parse_picker_output(stdout.Bytes())
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 && selected == "" {
			return "", Err_pick_cancelled
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("folder picker failed: %s", Sanitize(detail))
	}
	return validate_picked_dir(selected)
}

func pick_folder(ctx context.Context) (string, error) {
	home, _ := os.UserHomeDir()
	cmd, err := find_picker(runtime.GOOS, exec.LookPath, home)
	if err != nil {
		return "", err
	}
	return run_picker(ctx, cmd)
}
