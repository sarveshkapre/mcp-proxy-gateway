package record

import (
  "encoding/json"
  "testing"
)

func TestRedactor_RedactsKeysAnywhere(t *testing.T) {
  r, err := NewRedactor([]string{"token"}, nil)
  if err != nil {
    t.Fatalf("new redactor: %v", err)
  }

  in := json.RawMessage(`{"token":"abc","nested":{"token":"def"},"list":[{"token":"ghi"}]}`)
  out, err := r.Apply(in)
  if err != nil {
    t.Fatalf("apply: %v", err)
  }

  got := string(out)
  if got != `{"list":[{"token":"[REDACTED]"}],"nested":{"token":"[REDACTED]"},"token":"[REDACTED]"}` {
    t.Fatalf("unexpected redaction: %s", got)
  }
}

func TestRedactor_RedactsByRegex(t *testing.T) {
  r, err := NewRedactor(nil, []string{`(?i)secret|api[_-]?key`})
  if err != nil {
    t.Fatalf("new redactor: %v", err)
  }

  in := json.RawMessage(`{"apiKey":"k","SECRET":"s","ok":1}`)
  out, err := r.Apply(in)
  if err != nil {
    t.Fatalf("apply: %v", err)
  }

  got := string(out)
  if got != `{"SECRET":"[REDACTED]","apiKey":"[REDACTED]","ok":1}` {
    t.Fatalf("unexpected redaction: %s", got)
  }
}

func TestNewRedactor_InvalidRegex(t *testing.T) {
  if _, err := NewRedactor(nil, []string{"("}); err == nil {
    t.Fatalf("expected error")
  }
}

