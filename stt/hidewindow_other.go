//go:build !windows

package stt

import "os/exec"

func hideConsoleWindow(_ *exec.Cmd) {}
