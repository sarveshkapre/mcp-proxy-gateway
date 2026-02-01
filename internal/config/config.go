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
  return policy, nil
}
