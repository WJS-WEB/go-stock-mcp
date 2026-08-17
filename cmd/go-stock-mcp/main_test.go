package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNewToolBridgeRegistersReadOnlyPublicTools(t *testing.T) {
	registered, err := newToolBridge(context.Background())
	if err != nil {
		t.Fatalf("newToolBridge() error = %v", err)
	}
	if len(registered) != len(publicToolSpecs) {
		t.Fatalf("registered %d tools, want %d", len(registered), len(publicToolSpecs))
	}

	seen := make(map[string]struct{}, len(registered))
	for _, item := range registered {
		if item.definition.Name == "" {
			t.Fatal("registered tool has an empty MCP name")
		}
		if _, ok := seen[item.definition.Name]; ok {
			t.Fatalf("duplicate MCP tool name %q", item.definition.Name)
		}
		seen[item.definition.Name] = struct{}{}
		if item.definition.Annotations.ReadOnlyHint == nil || !*item.definition.Annotations.ReadOnlyHint {
			t.Errorf("tool %q is not marked read-only", item.definition.Name)
		}
		if item.definition.RawInputSchema == nil {
			t.Errorf("tool %q has no raw input schema", item.definition.Name)
		}
		var schema map[string]any
		if err := json.Unmarshal(item.definition.RawInputSchema, &schema); err != nil {
			t.Errorf("tool %q schema is invalid JSON: %v", item.definition.Name, err)
		}
	}
}

func TestToolInputSchemaForToolWithoutParameters(t *testing.T) {
	info := &schema.ToolInfo{}
	raw, err := toolInputSchema(info)
	if err != nil {
		t.Fatalf("toolInputSchema() error = %v", err)
	}
	if got := string(raw); got != `{"type":"object","properties":{}}` {
		t.Fatalf("toolInputSchema() = %s", got)
	}
}
