package language

import (
	"testing"
)

func TestLanguageForFile(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		filename string
		expected string
	}{
		{"main.py", "python"},
		{"app.js", "javascript"},
		{"component.tsx", "typescript"},
		{"index.ts", "typescript"},
		{"main.go", "go"},
		{"lib.rs", "rust"},
		{"MyClass.java", "java"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"main.cc", "cpp"},
		{"header.h", "c"},
		{"header.hpp", "cpp"},
		{"Program.cs", "csharp"},
		{"index.php", "php"},
		{"Main.scala", "scala"},
		{"app.swift", "swift"},
		{"main.dart", "dart"},
		{"Main.kt", "kotlin"},
		{"script.lua", "lua"},
		{"Main.hs", "haskell"},
		{"main.ml", "ocaml"},
		{"setup.sh", "bash"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"data.json", "json"},
		{"README.md", "markdown"},
		{"index.html", "html"},
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"style.sass", "scss"},
		{"query.sql", "sql"},
		{"file.proto", "proto"},
		{"Main.elm", "elm"},
		{"core.clj", "clojure"},
		{"app.ex", "elixir"},
		{"app.exs", "elixir"},
		{"Gemfile.rb", "ruby"},
		{"config.xml", "xml"},
		// Uppercase extensions.
		{"Main.PY", "python"},
		{"App.JS", "javascript"},
		// Unknown extensions.
		{"Makefile", ""},
		{"Dockerfile", ""},
		{"file.unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := r.LanguageForFile(tt.filename)
			if got != tt.expected {
				t.Errorf("LanguageForFile(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestLanguageForFileWithPath(t *testing.T) {
	r := NewRegistry()

	// Should extract just the extension from full paths.
	got := r.LanguageForFile("src/main/java/com/example/MyClass.java")
	if got != "java" {
		t.Errorf("LanguageForFile with path = %q, want java", got)
	}

	got = r.LanguageForFile("/absolute/path/to/file.rs")
	if got != "rust" {
		t.Errorf("LanguageForFile absolute = %q, want rust", got)
	}

	got = r.LanguageForFile(`C:\Users\test\file.go`)
	if got != "go" {
		t.Errorf("LanguageForFile Windows path = %q, want go", got)
	}
}

func TestIsLanguageAvailable(t *testing.T) {
	r := NewRegistry()

	if !r.IsLanguageAvailable("python") {
		t.Error("python should be available in new registry")
	}
	if r.IsLanguageAvailable("typescript") {
		t.Error("typescript should not be available until a parser is registered")
	}
}

func TestListAvailableLanguages(t *testing.T) {
	r := NewRegistry()

	langs := r.ListAvailableLanguages()
	if len(langs) != 11 {
		t.Errorf("new registry should have 11 bundled languages, got %d", len(langs))
	}
}

func TestGetLanguageNotRegistered(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetLanguage("typescript")
	if err == nil {
		t.Error("GetLanguage should fail for unregistered language")
	}
}

func TestGetParserNotRegistered(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetParser("typescript")
	if err == nil {
		t.Error("GetParser should fail for unregistered language")
	}
}
