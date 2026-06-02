package language

import (
	"testing"
)

func TestGetQueryTemplate(t *testing.T) {
	tests := []struct {
		language  string
		template  string
		wantEmpty bool
	}{
		// Existing templates.
		{"python", "functions", false},
		{"python", "classes", false},
		{"python", "imports", false},
		{"python", "function_calls", false},
		{"python", "assignments", false},
		{"javascript", "functions", false},
		{"javascript", "function_calls", false},
		{"javascript", "assignments", false},
		{"typescript", "interfaces", false},
		{"go", "interfaces", false},
		{"go", "structs", false},
		{"rust", "enums", false},
		{"rust", "traits", false},
		{"rust", "impls", false},
		{"java", "enums", false},
		{"java", "annotations", false},
		{"dart", "mixins", false},
		{"dart", "enums", false},
		{"c", "structs", false},
		{"cpp", "classes", false},
		{"swift", "structs", false},
		{"kotlin", "interfaces", false},
		{"csharp", "interfaces", false},
		{"xml", "elements", false},
		{"xml", "attributes", false},
		{"xml", "comments", false},
		// Non-existent templates.
		{"python", "nonexistent", true},
		{"javascript", "interfaces", true},
		{"c", "classes", true},
		// Non-existent languages.
		{"unknown_lang", "functions", true},
	}

	for _, tt := range tests {
		t.Run(tt.language+"/"+tt.template, func(t *testing.T) {
			got := GetQueryTemplate(tt.language, tt.template)
			if tt.wantEmpty && got != "" {
				t.Errorf("GetQueryTemplate(%q, %q) = %q, want empty", tt.language, tt.template, got)
			}
			if !tt.wantEmpty && got == "" {
				t.Errorf("GetQueryTemplate(%q, %q) returned empty, want non-empty", tt.language, tt.template)
			}
		})
	}
}

func TestListQueryTemplatesAll(t *testing.T) {
	result := ListQueryTemplates("")

	// Should return a map of language -> template names.
	if len(result) < 5 {
		t.Errorf("expected at least 5 languages, got %d", len(result))
	}

	// Check that Python templates include the new ones.
	pythonTemplates, ok := result["python"]
	if !ok {
		t.Fatal("python templates missing")
	}
	names, ok := pythonTemplates.([]string)
	if !ok {
		t.Fatal("python templates should be []string")
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"functions", "classes", "imports", "function_calls", "assignments"} {
		if !nameSet[expected] {
			t.Errorf("python templates should include %q", expected)
		}
	}
}

func TestListQueryTemplatesFiltered(t *testing.T) {
	result := ListQueryTemplates("rust")

	if len(result) != 1 {
		t.Errorf("expected exactly 1 entry for rust, got %d", len(result))
	}

	rustTemplates, ok := result["rust"]
	if !ok {
		t.Fatal("rust templates missing")
	}
	names, ok := rustTemplates.([]string)
	if !ok {
		t.Fatal("rust templates should be []string")
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"functions", "structs", "imports", "enums", "traits"} {
		if !nameSet[expected] {
			t.Errorf("rust templates should include %q", expected)
		}
	}
}

func TestListQueryTemplatesUnknownLang(t *testing.T) {
	result := ListQueryTemplates("unknown_lang")
	if len(result) != 0 {
		t.Errorf("unknown language should return empty, got %v", result)
	}
}

func TestAllLanguagesHaveBasicTemplates(t *testing.T) {
	// Languages that don't have traditional programming constructs.
	markupLangs := map[string]bool{"xml": true, "html": true, "json": true, "yaml": true, "markdown": true, "css": true, "scss": true, "sql": true, "proto": true}

	// Every language should have at least functions and imports templates.
	for lang := range QueryTemplates {
		if markupLangs[lang] {
			continue
		}
		if GetQueryTemplate(lang, "functions") == "" {
			t.Errorf("language %q missing 'functions' template", lang)
		}
		if GetQueryTemplate(lang, "imports") == "" {
			t.Errorf("language %q missing 'imports' template", lang)
		}
	}
}

func TestTemplateNames(t *testing.T) {
	tmpl := map[string]string{"a": "1", "b": "2", "c": "3"}
	names := templateNames(tmpl)
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}
}

func TestTemplateNamesEmpty(t *testing.T) {
	names := templateNames(map[string]string{})
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}
