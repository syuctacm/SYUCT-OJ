package handler

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/problem"
)

func TestNormalizeProblemConfigAcceptsHydroSchemaList(t *testing.T) {
	raw := "- key: type\n  value: default\n- key: time\n  value: 1000ms\n- key: memory\n  value: 256m\n"
	normalized, err := normalizeProblemConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(normalized, "type: default") || !strings.Contains(normalized, "time: 1000ms") {
		t.Fatalf("unexpected normalized config: %s", normalized)
	}
}

func TestProblemConfigStringConvertsBsonDocument(t *testing.T) {
	item := &problem.Problem{Config: bson.D{
		{Key: "type", Value: "default"},
		{Key: "time", Value: "1000ms"},
	}}
	rendered := problemConfigString(item)
	if !strings.Contains(rendered, "type: default") || !strings.Contains(rendered, "time: 1000ms") {
		t.Fatalf("unexpected rendered config: %s", rendered)
	}
}
