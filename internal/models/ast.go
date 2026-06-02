// Package models provides data types for ASTs and related entities.
package models

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// NodeToMap converts a tree-sitter node to a map representation.
func NodeToMap(node *sitter.Node, sourceBytes []byte, includeChildren bool, includeText bool, maxDepth int) map[string]any {
	if node == nil {
		return nil
	}

	sp := node.StartPosition()
	ep := node.EndPosition()

	result := map[string]any{
		"type": node.Kind(),
		"start_point": map[string]uint{
			"row":    sp.Row,
			"column": sp.Column,
		},
		"end_point": map[string]uint{
			"row":    ep.Row,
			"column": ep.Column,
		},
	}

	if includeText && sourceBytes != nil {
		result["text"] = node.Utf8Text(sourceBytes)
	}

	if includeChildren && maxDepth > 0 {
		children := make([]map[string]any, 0)
		cursor := &sitter.TreeCursor{}
		cursor.Reset(*node)

		if cursor.GotoFirstChild() {
			addChildNodes(cursor, sourceBytes, includeText, maxDepth-1, &children)
		}
		cursor.Close()
		result["children"] = children
		result["child_count"] = node.ChildCount()
	}

	return result
}

func addChildNodes(cursor *sitter.TreeCursor, sourceBytes []byte, includeText bool, maxDepth int, children *[]map[string]any) {
	for {
		node := cursor.Node()
		if node == nil {
			break
		}

		sp := node.StartPosition()
		ep := node.EndPosition()

		child := map[string]any{
			"type": node.Kind(),
			"start_point": map[string]uint{
				"row":    sp.Row,
				"column": sp.Column,
			},
			"end_point": map[string]uint{
				"row":    ep.Row,
				"column": ep.Column,
			},
		}

		if includeText && sourceBytes != nil {
			child["text"] = node.Utf8Text(sourceBytes)
		}

		if maxDepth > 0 && node.ChildCount() > 0 {
			grandchildren := make([]map[string]any, 0)
			childCursor := &sitter.TreeCursor{}
			childCursor.Reset(*node)
			if childCursor.GotoFirstChild() {
				addChildNodes(childCursor, sourceBytes, includeText, maxDepth-1, &grandchildren)
			}
			childCursor.Close()
			child["children"] = grandchildren
		}

		*children = append(*children, child)

		if !cursor.GotoNextSibling() {
			break
		}
	}
}

// SummarizeNode creates a compact summary of a node without children.
func SummarizeNode(node *sitter.Node, sourceBytes []byte) map[string]any {
	if node == nil {
		return nil
	}

	sp := node.StartPosition()
	ep := node.EndPosition()

	result := map[string]any{
		"type": node.Kind(),
		"start_point": map[string]uint{
			"row":    sp.Row,
			"column": sp.Column,
		},
		"end_point": map[string]uint{
			"row":    ep.Row,
			"column": ep.Column,
		},
	}

	if sourceBytes != nil {
		text := node.Utf8Text(sourceBytes)
		lines := splitLines(text)
		if len(lines) > 0 {
			snippet := lines[0]
			if len(snippet) > 50 {
				snippet = snippet[:50] + "..."
			} else if len(lines) > 1 {
				snippet += "..."
			}
			result["preview"] = snippet
		}
	}

	return result
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// FindNodeAtPosition finds the most specific node at a given row/column.
func FindNodeAtPosition(root *sitter.Node, row, col uint) *sitter.Node {
	if root == nil {
		return nil
	}
	point := sitter.Point{Row: row, Column: col}
	return root.DescendantForPointRange(point, point)
}
