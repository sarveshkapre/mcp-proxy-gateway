package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Policy struct {
	Version     int                  `json:"version" yaml:"version"`
	Mode        string               `json:"mode" yaml:"mode"`
	DefaultDeny bool                 `json:"default_deny" yaml:"default_deny"`
	AllowTools  []string             `json:"allow_tools" yaml:"allow_tools"`
	DenyTools   []string             `json:"deny_tools" yaml:"deny_tools"`
	Tools       map[string]ToolEntry `json:"tools" yaml:"tools"`
	Record      RecordPolicy         `json:"record" yaml:"record"`
	Replay      ReplayPolicy         `json:"replay" yaml:"replay"`
}

type RecordPolicy struct {
	RedactKeys     []string `json:"redact_keys" yaml:"redact_keys"`
	RedactKeyRegex []string `json:"redact_key_regex" yaml:"redact_key_regex"`

	// Optional recorder lifecycle controls.
	// max_bytes: rotate the active record file when the next append would exceed this size.
	// max_files: number of rotated backup files to retain (e.g. path.1..path.N).
	MaxBytes *int64 `json:"max_bytes" yaml:"max_bytes"`
	MaxFiles *int   `json:"max_files" yaml:"max_files"`
}

type ReplayPolicy struct {
	Match string `json:"match" yaml:"match"`
}

type ToolEntry struct {
	Schema map[string]any `json:"schema" yaml:"schema"`
}

func LoadPolicy(path string) (*Policy, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	policy := &Policy{}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, policy); err != nil {
			return nil, err
		}
	case ".json":
		if err := json.Unmarshal(data, policy); err != nil {
			return nil, err
		}
	default:
		if err := yaml.Unmarshal(data, policy); err != nil {
			return nil, err
		}
	}
	if policy.Version == 0 {
		policy.Version = 1
	}
	if policy.Mode == "" {
		policy.Mode = "enforce"
	}
	policy.Mode = strings.ToLower(policy.Mode)
	if policy.Mode != "enforce" && policy.Mode != "audit" && policy.Mode != "off" {
		return nil, errors.New("mode must be enforce, audit, or off")
	}
	if policy.Tools == nil {
		policy.Tools = map[string]ToolEntry{}
	}
	if policy.Replay.Match == "" {
		policy.Replay.Match = "signature"
	}
	policy.Replay.Match = strings.ToLower(policy.Replay.Match)
	if policy.Replay.Match != "signature" && policy.Replay.Match != "method" && policy.Replay.Match != "tool" {
		return nil, errors.New("replay.match must be signature, method, or tool")
	}

	if policy.Record.MaxBytes != nil && *policy.Record.MaxBytes < 0 {
		return nil, errors.New("record.max_bytes must be >= 0")
	}
	if policy.Record.MaxFiles != nil && *policy.Record.MaxFiles < 0 {
		return nil, errors.New("record.max_files must be >= 0")
	}
	return policy, nil
}
