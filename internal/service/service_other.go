//go:build !darwin && !linux

package service

import (
	"fmt"
	"runtime"
)

const manager = runtime.GOOS

// Supported reports whether this OS has a backend.
func Supported() bool { return false }

func unsupported() error {
	return fmt.Errorf("scheduled ingest is not supported on %s; schedule %q yourself (e.g. Task Scheduler)", runtime.GOOS, "tokenfetch ingest")
}

func Install(Config) (Info, error) { return Info{}, unsupported() }

func Uninstall() (bool, error) { return false, unsupported() }

func Status() (State, error) { return State{}, unsupported() }
