//go:build !mock

package main

import (
	"errors"
	"os/exec"
)

func checkDependencies() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("Error: 'gh' is required but not installed.")
	}
	return nil
}
