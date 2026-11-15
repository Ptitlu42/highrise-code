//go:build !windows

package cli

import (
	"os"
	"runtime"
)

func is_null_device(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	null, err := os.Stat(os.DevNull)
	if err != nil {
		return false
	}
	return os.SameFile(info, null)
}

func launched_from_gui(getenv func(string) string) bool {
	if is_terminal(os.Stdin) && !is_null_device(os.Stdin) {
		return false
	}
	if runtime.GOOS == "linux" {
		return getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != ""
	}
	return true
}
