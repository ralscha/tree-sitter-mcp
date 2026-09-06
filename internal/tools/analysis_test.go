package tools

import (
	"path/filepath"
	"testing"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected float64
	}{
		{
			name:     "identical sets",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: 1.0,
		},
		{
			name:     "disjoint sets",
			a:        []string{"a", "b", "c"},
			b:        []string{"d", "e", "f"},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			a:        []string{"a", "b", "c"},
			b:        []string{"b", "c", "d"},
			expected: 0.5,
		},
		{
			name:     "subset",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c", "d"},
			expected: 0.5,
		},
		{
			name:     "empty both",
			a:        []string{},
			b:        []string{},
			expected: 0.0,
		},
		{
			name:     "one empty",
			a:        []string{"a", "b"},
			b:        []string{},
			expected: 0.0,
		},
		{
			name:     "duplicates in both",
			a:        []string{"a", "a", "b", "b"},
			b:        []string{"a", "b", "b", "c"},
			expected: 0.6, // intersection: {a:1, b:2} = 3, union: {a:2, b:2, c:1} = 5 → 3/5 = 0.6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tt.a, tt.b)
			// Allow small floating-point tolerance.
			if got < tt.expected-0.001 || got > tt.expected+0.001 {
				t.Errorf("jaccardSimilarity(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestExtractSymbolsReturnsNamesWithoutOuterCaptureDuplicates(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "sample.go"), `package sample

type Thing struct{}

func Top() {}
func (Thing) Method() {}
`)
	project := &models.Project{RootPath: dir}
	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(100, 300)
	defer registry.Close()
	defer treeCache.Close()

	symbols, err := ExtractSymbols(project, "sample.go", registry, treeCache, []string{"functions"})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	for _, symbol := range symbols["functions"] {
		got[symbol.Name] = true
	}
	if len(symbols["functions"]) != 2 || !got["Top"] || !got["Method"] {
		t.Fatalf("functions = %#v, want exactly Top and Method", symbols["functions"])
	}
}

func TestCountLines(t *testing.T) {
	for _, test := range []struct {
		content string
		want    int
	}{
		{"", 0},
		{"one", 1},
		{"one\n", 1},
		{"one\ntwo", 2},
		{"one\ntwo\n", 2},
	} {
		if got := countLines([]byte(test.content)); got != test.want {
			t.Errorf("countLines(%q) = %d, want %d", test.content, got, test.want)
		}
	}
}

func TestJaccardSimilarityIsSymmetric(t *testing.T) {
	a := []string{"function_definition", "identifier", "block", "return_statement"}
	b := []string{"function_definition", "identifier", "block", "expression_statement"}

	ab := jaccardSimilarity(a, b)
	ba := jaccardSimilarity(b, a)

	if ab != ba {
		t.Errorf("jaccardSimilarity should be symmetric: got %v != %v", ab, ba)
	}
}

func TestDefaultSymbolTypes(t *testing.T) {
	tests := []struct {
		lang        string
		wantFuncs   bool
		wantImports bool
		wantStructs bool
		wantClasses bool
		wantEnums   bool
		wantTraits  bool
	}{
		{"rust", true, true, true, false, true, true},
		{"go", true, true, true, false, false, false},
		{"c", true, true, true, false, false, false},
		{"cpp", true, true, true, true, false, false},
		{"java", true, true, false, true, false, false},
		{"typescript", true, true, false, true, false, false},
		{"swift", true, true, true, true, false, false},
		{"dart", true, true, false, true, true, false},
		{"xml", false, false, false, false, false, false},
		{"python", true, true, false, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			types := defaultSymbolTypes(tt.lang)
			typeSet := make(map[string]bool)
			for _, tp := range types {
				typeSet[tp] = true
			}

			if typeSet["functions"] != tt.wantFuncs {
				t.Errorf("%s: functions = %v, want %v", tt.lang, typeSet["functions"], tt.wantFuncs)
			}
			if typeSet["imports"] != tt.wantImports {
				t.Errorf("%s: imports = %v, want %v", tt.lang, typeSet["imports"], tt.wantImports)
			}
			if typeSet["structs"] != tt.wantStructs {
				t.Errorf("%s: structs = %v, want %v", tt.lang, typeSet["structs"], tt.wantStructs)
			}
			if typeSet["classes"] != tt.wantClasses {
				t.Errorf("%s: classes = %v, want %v", tt.lang, typeSet["classes"], tt.wantClasses)
			}
			if typeSet["enums"] != tt.wantEnums {
				t.Errorf("%s: enums = %v, want %v", tt.lang, typeSet["enums"], tt.wantEnums)
			}
			if typeSet["traits"] != tt.wantTraits {
				t.Errorf("%s: traits = %v, want %v", tt.lang, typeSet["traits"], tt.wantTraits)
			}
		})
	}
}
