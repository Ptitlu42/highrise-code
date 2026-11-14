package cli

import (
	"errors"
	"os/exec"
	"runtime"

	"github.com/ptitlu42/highrise-code/internal/server"
)

func open_browser(url string) error {
	if !server.Valid_url(url) {
		return errors.New("refusing to open unexpected url")
	}
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	cmd := exec.Command(path, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
