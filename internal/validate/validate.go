package validate

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"

	"github.com/sarveshkapre/mcp-proxy-gateway/internal/config"
)

type Validator struct {
	mode        string
	defaultDeny bool
	allow       map[string]struct{}
	deny        map[string]struct{}
	schemas     map[string]*gojsonschema.Schema
}

type Decision struct {
	Allowed    bool
	Violations []string
}

func New(policy *config.Policy) (*Validator, error) {
	v := &Validator{
		mode:        "enforce",
		defaultDeny: false,
		allow:       map[string]struct{}{},
		deny:        map[string]struct{}{},
		schemas:     map[string]*gojsonschema.Schema{},
	}
	if policy == nil {
		v.mode = "off"
		return v, nil
	}
	v.mode = policy.Mode
	v.defaultDeny = policy.DefaultDeny
	for _, name := range policy.AllowTools {
		v.allow[name] = struct{}{}
	}
	for _, name := range policy.DenyTools {
		v.deny[name] = struct{}{}
	}
	for name, entry := range policy.Tools {
		if entry.Schema == nil {
			continue
		}
		schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(entry.Schema))
		if err != nil {
			return nil, fmt.Errorf("schema for %s: %w", name, err)
		}
		v.schemas[name] = schema
	}
	return v, nil
}

func (v *Validator) ValidateToolCall(tool string, args json.RawMessage) (Decision, error) {
	if v.mode == "off" {
		return Decision{Allowed: true}, nil
	}
	violations := []string{}

	if _, denied := v.deny[tool]; denied {
		violations = append(violations, "tool is denied")
	}

	if len(v.allow) > 0 {
		if _, ok := v.allow[tool]; !ok {
			violations = append(violations, "tool not in allowlist")
		}
	} else if v.defaultDeny {
		if _, ok := v.schemas[tool]; !ok {
			violations = append(violations, "tool not explicitly allowed")
		}
	}

	if schema, ok := v.schemas[tool]; ok {
		if len(args) == 0 {
			violations = append(violations, "arguments are required")
		} else {
			result, err := schema.Validate(gojsonschema.NewBytesLoader(args))
			if err != nil {
				return Decision{}, err
			}
			if !result.Valid() {
				for _, desc := range result.Errors() {
					violations = append(violations, desc.String())
				}
			}
		}
	}

	if len(violations) == 0 {
		return Decision{Allowed: true}, nil
	}

	if v.mode == "audit" {
		return Decision{Allowed: true, Violations: violations}, nil
	}

	return Decision{Allowed: false, Violations: violations}, nil
}
