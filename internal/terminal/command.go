package terminal

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Command describes an executable without shell syntax. Env is deliberately
// explicit: inherited secrets need never be serialized into terminal arguments.
type Command struct {
	Executable string
	Args       []string
	Dir        string
	Env        map[string]string
	Unset      []string
}

func (c Command) Validate() error {
	if strings.TrimSpace(c.Executable) == "" {
		return fmt.Errorf("empty executable")
	}
	for k, v := range c.Env {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return fmt.Errorf("invalid environment key/value %q", k)
		}
	}
	for _, k := range c.Unset {
		if k == "" || strings.ContainsAny(k, "=\x00") {
			return fmt.Errorf("invalid environment key %q", k)
		}
		if _, ok := c.Env[k]; ok {
			return fmt.Errorf("environment key %q both set and unset", k)
		}
	}
	return nil
}

// Argv forwards only explicit settings into a terminal's potentially different environment.
func (c Command) Argv() ([]string, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	args := []string{"/usr/bin/env"}
	unset := append([]string(nil), c.Unset...)
	sort.Strings(unset)
	for _, k := range unset {
		args = append(args, "-u", k)
	}
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, k+"="+c.Env[k])
	}
	args = append(args, c.Executable)
	return append(args, c.Args...), nil
}

func (c Command) Environment() []string {
	removed := make(map[string]bool)
	for _, k := range c.Unset {
		removed[k] = true
	}
	for k := range c.Env {
		removed[k] = true
	}
	env := make([]string, 0, len(os.Environ())+len(c.Env))
	for _, item := range os.Environ() {
		k, _, _ := strings.Cut(item, "=")
		if !removed[k] {
			env = append(env, item)
		}
	}
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+c.Env[k])
	}
	return env
}
