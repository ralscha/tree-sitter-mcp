// Package language provides tree-sitter language detection and parser management.
package language

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

var extToLang = map[string]string{
	"py":    "python",
	"js":    "javascript",
	"ts":    "typescript",
	"jsx":   "javascript",
	"tsx":   "typescript",
	"rb":    "ruby",
	"rs":    "rust",
	"go":    "go",
	"java":  "java",
	"c":     "c",
	"cpp":   "cpp",
	"cc":    "cpp",
	"h":     "c",
	"hpp":   "cpp",
	"cs":    "csharp",
	"php":   "php",
	"scala": "scala",
	"swift": "swift",
	"dart":  "dart",
	"kt":    "kotlin",
	"lua":   "lua",
	"hs":    "haskell",
	"ml":    "ocaml",
	"sh":    "bash",
	"yaml":  "yaml",
	"yml":   "yaml",
	"json":  "json",
	"md":    "markdown",
	"html":  "html",
	"css":   "css",
	"scss":  "scss",
	"sass":  "scss",
	"sql":   "sql",
	"proto": "proto",
	"elm":   "elm",
	"clj":   "clojure",
	"ex":    "elixir",
	"exs":   "elixir",
	"xml":   "xml",
}

// Registry manages tree-sitter language parsers.
type Registry struct {
	mu        sync.RWMutex
	languages map[string]*sitter.Language
}

// NewRegistry creates a new language registry.
func NewRegistry() *Registry {
	r := &Registry{
		languages: make(map[string]*sitter.Language),
	}

	r.RegisterLanguage("c", sitter.NewLanguage(tree_sitter_c.Language()))
	r.RegisterLanguage("cpp", sitter.NewLanguage(tree_sitter_cpp.Language()))
	r.RegisterLanguage("go", sitter.NewLanguage(tree_sitter_go.Language()))
	r.RegisterLanguage("html", sitter.NewLanguage(tree_sitter_html.Language()))
	r.RegisterLanguage("java", sitter.NewLanguage(tree_sitter_java.Language()))
	r.RegisterLanguage("javascript", sitter.NewLanguage(tree_sitter_javascript.Language()))
	r.RegisterLanguage("json", sitter.NewLanguage(tree_sitter_json.Language()))
	r.RegisterLanguage("php", sitter.NewLanguage(tree_sitter_php.LanguagePHP()))
	r.RegisterLanguage("python", sitter.NewLanguage(tree_sitter_python.Language()))
	r.RegisterLanguage("ruby", sitter.NewLanguage(tree_sitter_ruby.Language()))
	r.RegisterLanguage("rust", sitter.NewLanguage(tree_sitter_rust.Language()))

	return r
}

// LanguageForFile detects the language from a filename's extension.
func (r *Registry) LanguageForFile(filename string) string {
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	ext = strings.ToLower(ext)
	return extToLang[ext]
}

// RegisterLanguage registers a tree-sitter language.
func (r *Registry) RegisterLanguage(name string, lang *sitter.Language) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.languages[name] = lang
}

// Close releases parser resources held by the registry.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.languages = make(map[string]*sitter.Language)
}

// GetLanguage returns the tree-sitter Language for a given name.
func (r *Registry) GetLanguage(name string) (*sitter.Language, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lang, ok := r.languages[name]
	if !ok {
		return nil, fmt.Errorf("language not available: %s", name)
	}
	return lang, nil
}

// GetParser returns a tree-sitter Parser configured for the given language.
func (r *Registry) GetParser(name string) (*sitter.Parser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lang, ok := r.languages[name]
	if !ok {
		return nil, fmt.Errorf("language not available: %s", name)
	}
	parser := sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		parser.Close()
		return nil, fmt.Errorf("SetLanguage(%s): %w", name, err)
	}
	return parser, nil
}

// IsLanguageAvailable checks if a language is registered.
func (r *Registry) IsLanguageAvailable(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.languages[name]
	return ok
}

// ListAvailableLanguages returns all registered language names.
func (r *Registry) ListAvailableLanguages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.languages))
	for name := range r.languages {
		names = append(names, name)
	}
	return names
}
