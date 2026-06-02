// Package language provides query templates for common code patterns.
package language

// QueryTemplates holds tree-sitter query templates per language.
var QueryTemplates = map[string]map[string]string{
	"python": {
		"functions": `(function_definition name: (identifier) @function.name) @function`,
		"classes":   `(class_definition name: (identifier) @class.name) @class`,
		"imports": `[
			(import_statement name: (dotted_name) @import.name)
			(import_from_statement module_name: (dotted_name) @import.from)
		] @import`,
		"function_calls": `(call function: (identifier) @call.function arguments: (argument_list) @call.args) @call`,
		"assignments":    `(assignment left: (_) @assign.target right: (_) @assign.value) @assign`,
	},
	"javascript": {
		"functions": `[
			(function_declaration name: (identifier) @function.name)
			(arrow_function)
			(method_definition name: (property_identifier) @function.name)
		] @function`,
		"classes":        `(class_declaration name: (identifier) @class.name) @class`,
		"imports":        `(import_statement source: (string) @import.source) @import`,
		"function_calls": `(call_expression function: (identifier) @call.function arguments: (arguments) @call.args) @call`,
		"assignments":    `(variable_declarator name: (_) @assign.target value: (_) @assign.value) @assign`,
	},
	"typescript": {
		"functions": `[
			(function_declaration name: (identifier) @function.name)
			(arrow_function)
			(method_definition name: (property_identifier) @function.name)
		] @function`,
		"classes":        `(class_declaration name: (type_identifier) @class.name) @class`,
		"interfaces":     `(interface_declaration name: (type_identifier) @interface.name) @interface`,
		"imports":        `(import_statement source: (string) @import.source) @import`,
		"function_calls": `(call_expression function: (identifier) @call.function arguments: (arguments) @call.args) @call`,
		"assignments":    `(variable_declarator name: (_) @assign.target value: (_) @assign.value) @assign`,
	},
	"go": {
		"functions":  `(function_declaration name: (identifier) @function.name) @function`,
		"structs":    `(type_declaration (type_spec name: (type_identifier) @struct.name type: (struct_type)) @struct)`,
		"interfaces": `(type_declaration (type_spec name: (type_identifier) @interface.name type: (interface_type)) @interface)`,
		"imports":    `(import_spec path: (interpreted_string_literal) @import.path) @import`,
	},
	"rust": {
		"functions": `(function_item name: (identifier) @function.name) @function`,
		"structs":   `(struct_item name: (type_identifier) @struct.name) @struct`,
		"enums":     `(enum_item name: (type_identifier) @enum.name) @enum`,
		"traits":    `(trait_item name: (type_identifier) @trait.name) @trait`,
		"impls":     `(impl_item type: (_) @impl.type) @impl`,
		"imports":   `(use_declaration) @import`,
	},
	"java": {
		"functions": `(method_declaration name: (identifier) @function.name) @function`,
		"classes": `[
			(class_declaration name: (identifier) @class.name)
			(interface_declaration name: (identifier) @class.name)
		] @class`,
		"interfaces":  `(interface_declaration name: (identifier) @interface.name) @interface`,
		"enums":       `(enum_declaration name: (identifier) @enum.name) @enum`,
		"annotations": `(annotation name: (identifier) @annotation.name) @annotation`,
		"imports":     `(import_declaration) @import`,
	},
	"c": {
		"functions": `(function_definition declarator: (function_declarator declarator: (identifier) @function.name)) @function`,
		"structs":   `(struct_specifier name: (type_identifier) @struct.name) @struct`,
		"imports":   `(preproc_include path: (string_literal) @import.path) @import`,
	},
	"cpp": {
		"functions": `(function_definition declarator: (function_declarator declarator: (identifier) @function.name)) @function`,
		"classes":   `(class_specifier name: (type_identifier) @class.name) @class`,
		"structs":   `(struct_specifier name: (type_identifier) @struct.name) @struct`,
		"imports":   `(preproc_include path: (string_literal) @import.path) @import`,
	},
	"swift": {
		"functions": `(function_declaration name: (simple_identifier) @function.name) @function`,
		"classes":   `(class_declaration name: (type_identifier) @class.name) @class`,
		"structs":   `(struct_declaration name: (type_identifier) @struct.name) @struct`,
		"imports":   `(import_declaration) @import`,
	},
	"kotlin": {
		"functions":  `(function_declaration name: (simple_identifier) @function.name) @function`,
		"classes":    `(class_declaration name: (type_identifier) @class.name) @class`,
		"interfaces": `(interface_declaration name: (type_identifier) @interface.name) @interface`,
		"imports":    `(import_header) @import`,
	},
	"dart": {
		"functions": `(function_signature name: (identifier) @function.name) @function`,
		"classes":   `(class_definition name: (identifier) @class.name) @class`,
		"mixins":    `(mixin_definition name: (identifier) @mixin.name) @mixin`,
		"enums":     `(enum_definition name: (identifier) @enum.name) @enum`,
		"imports":   `(import_specification) @import`,
	},
	"csharp": {
		"functions":  `(method_declaration name: (identifier) @function.name) @function`,
		"classes":    `(class_declaration name: (identifier) @class.name) @class`,
		"interfaces": `(interface_declaration name: (identifier) @interface.name) @interface`,
		"imports":    `(using_directive) @import`,
	},
	"xml": {
		"elements":   `(element (start_tag (name) @element.name)) @element`,
		"attributes": `(attribute (attribute_name) @attr.name (quoted_attribute_value) @attr.value) @attribute`,
		"comments":   `(comment) @comment`,
	},
}

// GetQueryTemplate returns a query template for a language and template name.
func GetQueryTemplate(language, templateName string) string {
	if langTemplates, ok := QueryTemplates[language]; ok {
		return langTemplates[templateName]
	}
	return ""
}

// ListQueryTemplates lists all templates, optionally filtered by language.
func ListQueryTemplates(language string) map[string]any {
	if language != "" {
		if tmpl, ok := QueryTemplates[language]; ok {
			return map[string]any{language: templateNames(tmpl)}
		}
		return map[string]any{}
	}

	result := make(map[string]any)
	for lang, tmpl := range QueryTemplates {
		result[lang] = templateNames(tmpl)
	}
	return result
}

func templateNames(templates map[string]string) []string {
	names := make([]string, 0, len(templates))
	for name := range templates {
		names = append(names, name)
	}
	return names
}
