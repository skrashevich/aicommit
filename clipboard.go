package main

import (
	"errors"
	"os/exec"
	"strings"
)

func copyToClipboard(text string) error {
	candidates := []struct {
		name string
		args []string
	}{
		{name: "pbcopy"},
		{name: "wl-copy"},
		{name: "xclip", args: []string{"-selection", "clipboard"}},
		{name: "xsel", args: []string{"--clipboard", "--input"}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err != nil {
			continue
		}
		cmd := exec.Command(c.name, c.args...)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return errors.New("no clipboard command found")
}
