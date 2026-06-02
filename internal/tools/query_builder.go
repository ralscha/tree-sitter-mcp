// Package tools provides query building utilities for the MCP server.
package tools

import (
	"fmt"
	"strings"

	"tree-sitter-mcp/internal/language"
)

// BuildQuery combines multiple query templates into a compound query.
// combineMode can be "or" (union of matches) or "and" (requires all patterns to match).
func BuildQuery(lang string, patterns []string, combineMode string) (map[string]any, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns provided")
	}

	var queries []string
	for _, pattern := range patterns {
		tmpl := language.GetQueryTemplate(lang, pattern)
		if tmpl != "" {
			queries = append(queries, tmpl)
		} else if strings.Contains(pattern, "(") || strings.Contains(pattern, "@") {
			// Treat as a raw query string only if it looks like a query.
			queries = append(queries, pattern)
		}
		// Otherwise skip unrecognized pattern names for unsupported languages.
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("no valid queries for language '%s' with patterns %v", lang, patterns)
	}

	combined := strings.Join(queries, "\n")

	switch strings.ToLower(combineMode) {
	case "or":
		// For OR, just concatenate — tree-sitter matches any pattern.
	case "and":
		// For AND, we'd need predicates. Add a hint.
		combined += "\n\n;; Add #match? or #eq? predicates to require combinations of the above patterns."
	default:
		combined = strings.Join(queries, "\n")
	}

	return map[string]any{
		"language":    lang,
		"patterns":    patterns,
		"combine":     combineMode,
		"query":       combined,
		"num_queries": len(queries),
	}, nil
}

// AdaptQuery adapts a query from one language to another by translating node type names.
func AdaptQuery(query string, fromLang string, toLang string) (map[string]any, error) {
	translations := getNodeTypeTranslations()

	pair := fromLang + "->" + toLang
	revPair := toLang + "->" + fromLang

	trans, ok := translations[pair]
	if !ok {
		trans, ok = translations[revPair]
	}

	if !ok {
		// Return the query unchanged with a note.
		return map[string]any{
			"original_language": fromLang,
			"target_language":   toLang,
			"original_query":    query,
			"adapted_query":     query,
			"note":              fmt.Sprintf("no translations available for %s -> %s, query returned unchanged", fromLang, toLang),
		}, nil
	}

	adapted := query
	for from, to := range trans {
		adapted = strings.ReplaceAll(adapted, from, to)
	}

	return map[string]any{
		"original_language": fromLang,
		"target_language":   toLang,
		"original_query":    query,
		"adapted_query":     adapted,
		"translations":      trans,
	}, nil
}

// GetNodeTypes returns descriptions of common AST node types for a language.
func GetNodeTypes(lang string) (map[string]any, error) {
	descriptions := getNodeTypeDescriptions()

	desc, ok := descriptions[lang]
	if !ok {
		return nil, fmt.Errorf("no node type descriptions for language '%s'", lang)
	}

	return map[string]any{
		"language":   lang,
		"node_types": desc,
	}, nil
}

// ListAllNodeTypes returns available node type descriptions for all languages.
func ListAllNodeTypes() map[string]any {
	descriptions := getNodeTypeDescriptions()
	langs := make([]string, 0, len(descriptions))
	for lang := range descriptions {
		langs = append(langs, lang)
	}
	return map[string]any{
		"languages": langs,
	}
}

// getNodeTypeTranslations returns known translations between languages.
func getNodeTypeTranslations() map[string]map[string]string {
	return map[string]map[string]string{
		"python->javascript": {
			"function_definition": "function_declaration",
			"class_definition":    "class_declaration",
			"block":               "statement_block",
			"parameters":          "formal_parameters",
			"argument_list":       "arguments",
			"import_statement":    "import_statement",
			"call":                "call_expression",
		},
		"python->typescript": {
			"function_definition": "function_declaration",
			"class_definition":    "class_declaration",
			"block":               "statement_block",
			"parameters":          "formal_parameters",
			"argument_list":       "arguments",
			"call":                "call_expression",
		},
		"python->java": {
			"function_definition": "method_declaration",
			"class_definition":    "class_declaration",
			"block":               "block",
			"parameters":          "formal_parameters",
			"argument_list":       "argument_list",
		},
		"python->go": {
			"function_definition": "function_declaration",
			"class_definition":    "type_declaration",
			"block":               "block",
			"parameters":          "parameter_list",
		},
		"python->rust": {
			"function_definition": "function_item",
			"class_definition":    "struct_item",
			"block":               "block",
			"parameters":          "parameters",
		},
		"javascript->python": {
			"function_declaration": "function_definition",
			"class_declaration":    "class_definition",
			"statement_block":      "block",
			"formal_parameters":    "parameters",
			"arguments":            "argument_list",
			"call_expression":      "call",
		},
		"typescript->python": {
			"function_declaration": "function_definition",
			"class_declaration":    "class_definition",
			"statement_block":      "block",
			"formal_parameters":    "parameters",
			"arguments":            "argument_list",
			"call_expression":      "call",
		},
		"java->python": {
			"method_declaration": "function_definition",
			"class_declaration":  "class_definition",
			"block":              "block",
		},
		"go->python": {
			"function_declaration": "function_definition",
			"type_declaration":     "class_definition",
			"block":                "block",
			"parameter_list":       "parameters",
		},
		"go->rust": {
			"function_declaration": "function_item",
			"type_declaration":     "struct_item",
			"block":                "block",
		},
		"go->javascript": {
			"function_declaration": "function_declaration",
			"type_declaration":     "class_declaration",
			"block":                "statement_block",
		},
		"go->java": {
			"function_declaration": "method_declaration",
			"type_declaration":     "class_declaration",
			"block":                "block",
		},
		"rust->python": {
			"function_item": "function_definition",
			"struct_item":   "class_definition",
			"block":         "block",
			"parameters":    "parameters",
		},
	}
}

// getNodeTypeDescriptions returns human-readable descriptions of common node types per language.
func getNodeTypeDescriptions() map[string]map[string]string {
	return map[string]map[string]string{
		"python": {
			"function_definition": "A function definition with `def` keyword, name, parameters, and body.",
			"class_definition":    "A class definition with `class` keyword, name, optional bases, and body.",
			"import_statement":    "An import statement (`import x` or `from x import y`).",
			"call":                "A function or method call expression.",
			"assignment":          "An assignment statement with a target and value.",
			"if_statement":        "An if/elif/else conditional statement.",
			"for_statement":       "A for loop with target, iterable, and body.",
			"while_statement":     "A while loop with condition and body.",
			"try_statement":       "A try/except/finally block.",
			"return_statement":    "A return statement with optional value.",
			"decorator":           "A decorator applied to a function or class.",
			"identifier":          "Any identifier (variable name, function name, etc.).",
			"string":              "A string literal.",
			"block":               "An indented block of statements.",
		},
		"javascript": {
			"function_declaration":  "A function declaration with `function` keyword.",
			"arrow_function":        "An arrow function expression `() => {}`.",
			"class_declaration":     "A class declaration with `class` keyword.",
			"method_definition":     "A method definition inside a class or object.",
			"import_statement":      "An ES6 import statement.",
			"call_expression":       "A function or method call.",
			"assignment_expression": "An assignment with `=` operator.",
			"if_statement":          "An if/else conditional.",
			"for_statement":         "A for loop.",
			"try_statement":         "A try/catch/finally block.",
			"return_statement":      "A return statement.",
			"variable_declarator":   "A variable declaration with `let`, `const`, or `var`.",
			"identifier":            "Any identifier.",
			"string":                "A string literal.",
			"statement_block":       "A block of statements in `{}`.",
		},
		"typescript": {
			"function_declaration":   "A function declaration with optional type annotations.",
			"arrow_function":         "An arrow function expression.",
			"class_declaration":      "A class declaration.",
			"interface_declaration":  "A TypeScript interface declaration.",
			"type_alias_declaration": "A `type` alias declaration.",
			"enum_declaration":       "An enum declaration.",
			"import_statement":       "An ES6 import statement.",
			"call_expression":        "A function or method call.",
			"identifier":             "Any identifier.",
			"type_identifier":        "A type name identifier.",
			"statement_block":        "A block of statements in `{}`.",
		},
		"go": {
			"function_declaration":       "A function declaration with `func` keyword.",
			"method_declaration":         "A method declaration with receiver.",
			"type_declaration":           "A type declaration (struct, interface, or alias).",
			"struct_type":                "A struct type definition.",
			"interface_type":             "An interface type definition.",
			"import_declaration":         "An import declaration.",
			"call_expression":            "A function call.",
			"if_statement":               "An if/else statement.",
			"for_statement":              "A for loop.",
			"return_statement":           "A return statement.",
			"short_var_declaration":      "A `:=` short variable declaration.",
			"identifier":                 "Any identifier.",
			"type_identifier":            "A type name.",
			"interpreted_string_literal": "A string literal.",
			"block":                      "A block of statements in `{}`.",
		},
		"rust": {
			"function_item":     "A function definition with `fn` keyword.",
			"struct_item":       "A struct definition.",
			"enum_item":         "An enum definition.",
			"impl_item":         "An `impl` block.",
			"trait_item":        "A trait definition.",
			"use_declaration":   "A `use` import statement.",
			"call_expression":   "A function call.",
			"let_declaration":   "A `let` binding.",
			"if_expression":     "An if/else expression.",
			"for_expression":    "A for loop expression.",
			"return_expression": "A return expression.",
			"identifier":        "Any identifier.",
			"type_identifier":   "A type name.",
			"block":             "A block of statements in `{}`.",
		},
		"java": {
			"method_declaration":      "A method declaration inside a class.",
			"class_declaration":       "A class declaration.",
			"interface_declaration":   "An interface declaration.",
			"constructor_declaration": "A constructor declaration.",
			"field_declaration":       "A field/variable declaration.",
			"import_declaration":      "An import statement.",
			"method_invocation":       "A method call.",
			"if_statement":            "An if/else statement.",
			"for_statement":           "A for loop.",
			"return_statement":        "A return statement.",
			"identifier":              "Any identifier.",
			"type_identifier":         "A type name.",
			"string_literal":          "A string literal.",
			"block":                   "A block of statements in `{}`.",
		},
		"cpp": {
			"function_definition": "A function definition.",
			"class_specifier":     "A class definition.",
			"struct_specifier":    "A struct definition.",
			"field_declaration":   "A field declaration.",
			"preproc_include":     "A `#include` directive.",
			"call_expression":     "A function call.",
			"if_statement":        "An if/else statement.",
			"for_statement":       "A for loop.",
			"return_statement":    "A return statement.",
			"identifier":          "Any identifier.",
			"type_identifier":     "A type name.",
			"compound_statement":  "A block of statements in `{}`.",
		},
		"c": {
			"function_definition": "A function definition.",
			"struct_specifier":    "A struct definition.",
			"preproc_include":     "A `#include` directive.",
			"call_expression":     "A function call.",
			"if_statement":        "An if/else statement.",
			"for_statement":       "A for loop.",
			"return_statement":    "A return statement.",
			"identifier":          "Any identifier.",
			"type_identifier":     "A type name.",
			"compound_statement":  "A block of statements in `{}`.",
		},
		"swift": {
			"function_declaration": "A function declaration with `func` keyword.",
			"class_declaration":    "A class declaration.",
			"struct_declaration":   "A struct declaration.",
			"protocol_declaration": "A protocol declaration.",
			"import_declaration":   "An import statement.",
			"call_expression":      "A function call.",
			"if_statement":         "An if/else statement.",
			"for_statement":        "A for loop.",
			"return_statement":     "A return statement.",
			"identifier":           "Any identifier.",
			"type_identifier":      "A type name.",
		},
		"kotlin": {
			"function_declaration": "A function declaration with `fun` keyword.",
			"class_declaration":    "A class declaration.",
			"object_declaration":   "An object declaration.",
			"import_header":        "An import statement.",
			"call_expression":      "A function call.",
			"if_expression":        "An if/else expression.",
			"for_expression":       "A for loop.",
			"return_expression":    "A return expression.",
			"identifier":           "Any identifier.",
			"type_identifier":      "A type name.",
		},
		"dart": {
			"function_declaration": "A function declaration.",
			"class_declaration":    "A class declaration.",
			"mixin_declaration":    "A mixin declaration.",
			"enum_declaration":     "An enum declaration.",
			"import_specification": "An import statement.",
			"method_invocation":    "A method call.",
			"if_statement":         "An if/else statement.",
			"for_statement":        "A for loop.",
			"return_statement":     "A return statement.",
			"identifier":           "Any identifier.",
			"type_identifier":      "A type name.",
		},
		"xml": {
			"document":               "The root XML document node.",
			"element":                "An XML element with optional start tag, content, and end tag.",
			"start_tag":              "An opening tag like `<name>`.",
			"end_tag":                "A closing tag like `</name>`.",
			"self_closing_tag":       "A self-closing tag like `<name/>`.",
			"name":                   "The tag name of an element.",
			"attribute":              "An attribute within a start tag.",
			"attribute_name":         "The name part of an attribute.",
			"quoted_attribute_value": "The quoted value of an attribute.",
			"text":                   "Text content between XML tags.",
			"comment":                "An XML comment (`<!-- ... -->`).",
			"processing_instruction": "A processing instruction (`<? ... ?>`).",
			"doctype":                "A DOCTYPE declaration.",
			"cdata":                  "A CDATA section.",
		},
		"html": {
			"element":          "An HTML element.",
			"start_tag":        "An opening tag.",
			"end_tag":          "A closing tag.",
			"self_closing_tag": "A self-closing tag.",
			"attribute":        "An HTML attribute.",
			"text":             "Text content.",
			"comment":          "An HTML comment.",
			"doctype":          "A DOCTYPE declaration.",
			"style_element":    "A `<style>` element.",
			"script_element":   "A `<script>` element.",
		},
	}
}
