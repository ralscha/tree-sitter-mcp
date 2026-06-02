package tools

import (
	"strings"
	"testing"
)

func TestBuildQueryOr(t *testing.T) {
	result, err := BuildQuery("python", []string{"functions", "classes"}, "or")
	if err != nil {
		t.Fatalf("BuildQuery failed: %v", err)
	}

	query, ok := result["query"].(string)
	if !ok {
		t.Fatal("query field missing")
	}

	if !strings.Contains(query, "function_definition") {
		t.Error("query should contain function_definition pattern")
	}
	if !strings.Contains(query, "class_definition") {
		t.Error("query should contain class_definition pattern")
	}

	if result["language"] != "python" {
		t.Errorf("language = %v, want python", result["language"])
	}
	if result["combine"] != "or" {
		t.Errorf("combine = %v, want or", result["combine"])
	}
	numQueries, _ := result["num_queries"].(int)
	if numQueries != 2 {
		t.Errorf("num_queries = %v, want 2", numQueries)
	}
}

func TestBuildQueryAnd(t *testing.T) {
	result, err := BuildQuery("javascript", []string{"functions", "classes"}, "and")
	if err != nil {
		t.Fatalf("BuildQuery failed: %v", err)
	}

	query, _ := result["query"].(string)
	if !strings.Contains(query, "function_declaration") {
		t.Error("query should contain function_declaration pattern")
	}
	if !strings.Contains(query, "class_declaration") {
		t.Error("query should contain class_declaration pattern")
	}
	if result["combine"] != "and" {
		t.Errorf("combine = %v, want and", result["combine"])
	}
}

func TestBuildQueryRaw(t *testing.T) {
	// Test with a raw query string as one of the patterns.
	result, err := BuildQuery("python", []string{"functions", "(call) @call"}, "or")
	if err != nil {
		t.Fatalf("BuildQuery failed: %v", err)
	}

	query, _ := result["query"].(string)
	if !strings.Contains(query, "(call) @call") {
		t.Error("query should contain raw (call) pattern")
	}
}

func TestBuildQueryEmptyPatterns(t *testing.T) {
	_, err := BuildQuery("python", []string{}, "or")
	if err == nil {
		t.Error("BuildQuery should fail with empty patterns")
	}
}

func TestBuildQueryInvalidLanguage(t *testing.T) {
	_, err := BuildQuery("unknown_lang", []string{"functions"}, "or")
	if err == nil {
		t.Error("BuildQuery should fail for unsupported language with no raw query")
	}
}

func TestBuildQueryUnknownPatternFallsBackToRaw(t *testing.T) {
	result, err := BuildQuery("python", []string{"(call) @call"}, "or")
	if err != nil {
		t.Fatalf("BuildQuery failed: %v", err)
	}
	query, _ := result["query"].(string)
	if !strings.Contains(query, "(call) @call") {
		t.Error("raw query should be passed through")
	}
}

func TestAdaptQueryWithTranslations(t *testing.T) {
	result, err := AdaptQuery(
		"(function_definition name: (identifier) @name)",
		"python", "javascript",
	)
	if err != nil {
		t.Fatalf("AdaptQuery failed: %v", err)
	}

	adapted, _ := result["adapted_query"].(string)
	if !strings.Contains(adapted, "function_declaration") {
		t.Errorf("adapted query should use function_declaration, got: %s", adapted)
	}
	if result["original_language"] != "python" {
		t.Errorf("original_language = %v, want python", result["original_language"])
	}
	if result["target_language"] != "javascript" {
		t.Errorf("target_language = %v, want javascript", result["target_language"])
	}
}

func TestAdaptQueryNoTranslations(t *testing.T) {
	result, err := AdaptQuery(
		"(function_definition name: (identifier) @name)",
		"python", "lua",
	)
	if err != nil {
		t.Fatalf("AdaptQuery failed: %v", err)
	}

	adapted, _ := result["adapted_query"].(string)
	// Should return unchanged.
	if adapted != "(function_definition name: (identifier) @name)" {
		t.Errorf("query should be unchanged when no translations: got %s", adapted)
	}

	note, _ := result["note"].(string)
	if note == "" {
		t.Error("note should be present when no translations available")
	}
}

func TestAdaptQueryReverseDirection(t *testing.T) {
	// Test adapting from JavaScript back to Python (reverse translation).
	result, err := AdaptQuery(
		"(function_declaration) @func",
		"javascript", "python",
	)
	if err != nil {
		t.Fatalf("AdaptQuery failed: %v", err)
	}

	adapted, _ := result["adapted_query"].(string)
	if !strings.Contains(adapted, "function_definition") {
		t.Errorf("adapted query should use function_definition, got: %s", adapted)
	}
}

func TestGetNodeTypesValid(t *testing.T) {
	result, err := GetNodeTypes("python")
	if err != nil {
		t.Fatalf("GetNodeTypes failed: %v", err)
	}

	if result["language"] != "python" {
		t.Errorf("language = %v, want python", result["language"])
	}

	nodeTypes, ok := result["node_types"].(map[string]string)
	if !ok {
		t.Fatal("node_types missing or wrong type")
	}

	// Check for common Python node types.
	for _, key := range []string{"function_definition", "class_definition", "import_statement", "call"} {
		if _, ok := nodeTypes[key]; !ok {
			t.Errorf("expected node type %q for python", key)
		}
	}
}

func TestGetNodeTypesInvalid(t *testing.T) {
	_, err := GetNodeTypes("nonexistent_language")
	if err == nil {
		t.Error("GetNodeTypes should fail for unsupported language")
	}
}

func TestListAllNodeTypes(t *testing.T) {
	result := ListAllNodeTypes()
	languages, ok := result["languages"].([]string)
	if !ok {
		t.Fatal("languages field missing or wrong type")
	}

	if len(languages) < 5 {
		t.Errorf("expected at least 5 languages, got %d", len(languages))
	}

	// Verify some expected languages are present.
	langSet := make(map[string]bool)
	for _, lang := range languages {
		langSet[lang] = true
	}
	for _, expected := range []string{"python", "javascript", "go", "rust", "java", "xml"} {
		if !langSet[expected] {
			t.Errorf("expected language %q in list", expected)
		}
	}
}

func TestAdaptQueryGoToRust(t *testing.T) {
	result, err := AdaptQuery(
		"(function_declaration) @func",
		"go", "rust",
	)
	if err != nil {
		t.Fatalf("AdaptQuery failed: %v", err)
	}

	adapted, _ := result["adapted_query"].(string)
	if !strings.Contains(adapted, "function_item") {
		t.Errorf("Go->Rust should translate function_declaration→function_item, got: %s", adapted)
	}
}

func TestBuildQueryDefaultCombine(t *testing.T) {
	// When no combine mode is specified, should default to OR-like behavior.
	result, err := BuildQuery("python", []string{"functions", "classes"}, "")
	if err != nil {
		t.Fatalf("BuildQuery failed: %v", err)
	}

	query, _ := result["query"].(string)
	if !strings.Contains(query, "function_definition") {
		t.Error("query should contain function_definition")
	}
	if !strings.Contains(query, "class_definition") {
		t.Error("query should contain class_definition")
	}
}
