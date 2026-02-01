package record

import (
  "encoding/json"
  "fmt"
  "regexp"
)

const defaultRedactionReplacement = "[REDACTED]"

type Redactor struct {
  keys        map[string]struct{}
  keyRegex    []*regexp.Regexp
  replacement string
}

func NewRedactor(redactKeys, redactKeyRegex []string) (*Redactor, error) {
  if len(redactKeys) == 0 && len(redactKeyRegex) == 0 {
    return nil, nil
  }

  r := &Redactor{
    keys:        map[string]struct{}{},
    replacement: defaultRedactionReplacement,
  }
  for _, k := range redactKeys {
    if k == "" {
      continue
    }
    r.keys[k] = struct{}{}
  }
  for _, pattern := range redactKeyRegex {
    if pattern == "" {
      continue
    }
    re, err := regexp.Compile(pattern)
    if err != nil {
      return nil, fmt.Errorf("invalid redact_key_regex %q: %w", pattern, err)
    }
    r.keyRegex = append(r.keyRegex, re)
  }
  if len(r.keys) == 0 && len(r.keyRegex) == 0 {
    return nil, nil
  }
  return r, nil
}

func (r *Redactor) Apply(raw json.RawMessage) (json.RawMessage, error) {
  if r == nil || len(raw) == 0 {
    return raw, nil
  }
  var v any
  if err := json.Unmarshal(raw, &v); err != nil {
    return nil, err
  }
  r.redactValue(v)
  out, err := json.Marshal(v)
  if err != nil {
    return nil, err
  }
  return json.RawMessage(out), nil
}

func (r *Redactor) redactValue(v any) {
  switch vv := v.(type) {
  case map[string]any:
    for k, child := range vv {
      if r.matchesKey(k) {
        vv[k] = r.replacement
        continue
      }
      r.redactValue(child)
    }
  case []any:
    for i := range vv {
      r.redactValue(vv[i])
    }
  default:
    return
  }
}

func (r *Redactor) matchesKey(k string) bool {
  if _, ok := r.keys[k]; ok {
    return true
  }
  for _, re := range r.keyRegex {
    if re.MatchString(k) {
      return true
    }
  }
  return false
}

