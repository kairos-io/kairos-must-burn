//go:build linux

package main

import (
	"os"
)

func CheckElevatedPermissions() (bool, error) {
	return os.Geteuid() == 0, nil
}
