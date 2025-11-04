//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func reallyBurn(isoPath, devicePath string, totalSize int64, progress *gtk.ProgressBar, status *gtk.Label) error {
	// Open ISO file for reading
	isoFile, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("failed to open ISO file: %w", err)
	}
	defer func() {
		if err := isoFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing ISO file: %v\n", err)
		}
	}()

	// Open device file for writing
	deviceFile, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open device %s: %w", devicePath, err)
	}
	defer func() {
		if err := deviceFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing device file: %v\n", err)
		}
	}()

	// Copy with progress tracking
	return copyWithProgress(isoFile, deviceFile, totalSize, progress, status)
}

func Sync() {
	syscall.Sync()
}

func FormatDriveGPT(deviceID string) error {
	for i := 1; i <= 16; i++ {
		part := fmt.Sprintf("%s%d", deviceID, i)
		if _, err := os.Stat(part); err == nil {
			_ = exec.Command("umount", part)
		}
	}

	// Zero first 34 sectors (17 KiB)
	f, err := os.OpenFile(deviceID, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open device: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing device: %v\n", err)
		}
	}()
	buf := make([]byte, 34*512) // 34 sectors * 512 bytes
	if _, err := f.WriteAt(buf, 0); err != nil {
		return fmt.Errorf("failed to write zeros: %w", err)
	}
	Sync()
	return nil
}
