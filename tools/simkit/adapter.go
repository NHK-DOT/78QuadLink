package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Adapter manifests describe integration, never executable shell fragments.
// A matching producer/consumer implementation must exist outside this toolkit.
type adapter struct {
	Schema        int                          `json:"schema"`
	ID            string                       `json:"id"`
	Description   string                       `json:"description"`
	BoardABI      string                       `json:"board_abi"`
	PayloadSchema string                       `json:"payload_schema"`
	BackendCLI    string                       `json:"backend_cli"`
	BoardEnv      string                       `json:"board_env"`
	ClearEnv      []string                     `json:"clear_env"`
	Profiles      map[string]map[string]string `json:"profiles"`
}

var envName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func loadAdapter(path string) (adapter, error) {
	var a adapter
	f, err := os.Open(path)
	if err != nil {
		return a, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(&a); err != nil {
		return a, err
	}
	var extra interface{}
	if err = d.Decode(&extra); err != io.EOF {
		return a, fmt.Errorf("adapter must contain one JSON object")
	}
	if a.Schema != 1 || a.ID == "" || a.BoardABI == "" || a.PayloadSchema == "" || a.BackendCLI != "go1relay-flags-v1" || !envName.MatchString(a.BoardEnv) || len(a.Profiles) == 0 {
		return a, fmt.Errorf("invalid adapter metadata or unsupported backend CLI")
	}
	for _, key := range a.ClearEnv {
		if !envName.MatchString(key) {
			return a, fmt.Errorf("invalid clear_env key %q", key)
		}
	}
	for name, env := range a.Profiles {
		if name == "" {
			return a, fmt.Errorf("empty profile name")
		}
		for key, value := range env {
			if !envName.MatchString(key) || key == a.BoardEnv || strings.ContainsRune(value, 0) {
				return a, fmt.Errorf("invalid profile environment key/value %q", key)
			}
		}
	}
	return a, nil
}

// Clear every managed key, including keys from unselected profiles. Ambient mode
// flags must not silently change the experiment requested by the user.
func (a adapter) environment(base []string, profile, board string) ([]string, error) {
	selected, ok := a.Profiles[profile]
	if !ok {
		return nil, fmt.Errorf("unknown profile %q", profile)
	}
	env := make(map[string]string)
	for _, v := range base {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	for _, key := range a.ClearEnv {
		delete(env, key)
	}
	for _, values := range a.Profiles {
		for key := range values {
			delete(env, key)
		}
	}
	delete(env, a.BoardEnv)
	env[a.BoardEnv] = board
	for key, value := range selected {
		env[key] = value
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+env[key])
	}
	return result, nil
}
