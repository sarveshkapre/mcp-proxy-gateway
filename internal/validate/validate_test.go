package validate

import (
  "encoding/json"
  "testing"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/config"
)

func TestValidatorEnforce(t *testing.T) {
  policy := &config.Policy{
    Mode: "enforce",
    Tools: map[string]config.ToolEntry{
      "web.search": {
        Schema: map[string]any{
          "type": "object",
          "properties": map[string]any{
            "query": map[string]any{"type": "string"},
          },
          "required":             []any{"query"},
          "additionalProperties": false,
        },
      },
    },
  }

  v, err := New(policy)
  if err != nil {
    t.Fatalf("validator init: %v", err)
  }

  args := json.RawMessage(`{"query":"hello"}`)
  decision, err := v.ValidateToolCall("web.search", args)
  if err != nil {
    t.Fatalf("validate: %v", err)
  }
  if !decision.Allowed {
    t.Fatalf("expected allowed")
  }

  badArgs := json.RawMessage(`{"nope":1}`)
  decision, err = v.ValidateToolCall("web.search", badArgs)
  if err != nil {
    t.Fatalf("validate: %v", err)
  }
  if decision.Allowed {
    t.Fatalf("expected rejection")
  }
}

func TestValidatorAudit(t *testing.T) {
  policy := &config.Policy{
    Mode: "audit",
    Tools: map[string]config.ToolEntry{
      "web.search": {
        Schema: map[string]any{
          "type": "object",
          "properties": map[string]any{
            "query": map[string]any{"type": "string"},
          },
          "required":             []any{"query"},
          "additionalProperties": false,
        },
      },
    },
  }

  v, err := New(policy)
  if err != nil {
    t.Fatalf("validator init: %v", err)
  }

  badArgs := json.RawMessage(`{"nope":1}`)
  decision, err := v.ValidateToolCall("web.search", badArgs)
  if err != nil {
    t.Fatalf("validate: %v", err)
  }
  if !decision.Allowed {
    t.Fatalf("expected allowed in audit")
  }
  if len(decision.Violations) == 0 {
    t.Fatalf("expected violations")
  }
}

func TestValidatorAllowList(t *testing.T) {
  policy := &config.Policy{
    Mode:       "enforce",
    AllowTools: []string{"fs.read"},
  }

  v, err := New(policy)
  if err != nil {
    t.Fatalf("validator init: %v", err)
  }

  decision, err := v.ValidateToolCall("web.search", json.RawMessage(`{"query":"hi"}`))
  if err != nil {
    t.Fatalf("validate: %v", err)
  }
  if decision.Allowed {
    t.Fatalf("expected rejection for tool not in allowlist")
  }
}
