package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/pkg/utils"
)

// treeNode is a test source entity for BuildTree
type treeNode struct {
	ID       uint
	ParentID uint
	Name     string
}

// treeDTO is the target tree node type
type treeDTO struct {
	ID       uint      `json:"id"`
	Name     string    `json:"name"`
	Children []treeDTO `json:"children"`
}

func TestBuildTree(t *testing.T) {
	convert := func(node treeNode, children []treeDTO) (treeDTO, bool) {
		return treeDTO{
			ID:       node.ID,
			Name:     node.Name,
			Children: children,
		}, true
	}

	t.Run("simple three-level hierarchy", func(t *testing.T) {
		elements := []treeNode{
			{ID: 1, ParentID: 0, Name: "Root"},
			{ID: 2, ParentID: 1, Name: "Child A"},
			{ID: 3, ParentID: 1, Name: "Child B"},
			{ID: 4, ParentID: 2, Name: "Grandchild A1"},
		}

		result := utils.BuildTree(elements,
			func(n treeNode) uint { return n.ParentID },
			func(n treeNode) uint { return n.ID },
			convert,
		)

		assert.Len(t, result, 1)
		assert.Equal(t, "Root", result[0].Name)
		assert.Len(t, result[0].Children, 2)

		// Find Child A
		var childA *treeDTO
		for i := range result[0].Children {
			if result[0].Children[i].Name == "Child A" {
				childA = &result[0].Children[i]
			}
		}
		assert.NotNil(t, childA)
		assert.Len(t, childA.Children, 1)
		assert.Equal(t, "Grandchild A1", childA.Children[0].Name)
	})

	t.Run("multiple roots", func(t *testing.T) {
		elements := []treeNode{
			{ID: 1, ParentID: 0, Name: "Root1"},
			{ID: 2, ParentID: 0, Name: "Root2"},
			{ID: 3, ParentID: 1, Name: "Child1"},
		}

		result := utils.BuildTree(elements,
			func(n treeNode) uint { return n.ParentID },
			func(n treeNode) uint { return n.ID },
			convert,
		)

		assert.Len(t, result, 2)
	})

	t.Run("empty input", func(t *testing.T) {
		result := utils.BuildTree([]treeNode{},
			func(n treeNode) uint { return n.ParentID },
			func(n treeNode) uint { return n.ID },
			convert,
		)
		assert.Nil(t, result)
	})

	t.Run("filter drops nodes", func(t *testing.T) {
		elements := []treeNode{
			{ID: 1, ParentID: 0, Name: "Root"},
			{ID: 2, ParentID: 1, Name: "Keep"},
			{ID: 3, ParentID: 1, Name: "Drop"},
		}

		filterConvert := func(node treeNode, children []treeDTO) (treeDTO, bool) {
			if node.Name == "Drop" {
				return treeDTO{}, false
			}
			return treeDTO{ID: node.ID, Name: node.Name, Children: children}, true
		}

		result := utils.BuildTree(elements,
			func(n treeNode) uint { return n.ParentID },
			func(n treeNode) uint { return n.ID },
			filterConvert,
		)

		assert.Len(t, result, 1)
		assert.Len(t, result[0].Children, 1)
		assert.Equal(t, "Keep", result[0].Children[0].Name)
	})

	t.Run("circular reference protection", func(t *testing.T) {
		// Node 2 references Node 1, Node 1 references Node 2 (cycle)
		// But both have non-zero parentID, so neither is a root
		// This should not cause infinite recursion
		elements := []treeNode{
			{ID: 1, ParentID: 2, Name: "A"},
			{ID: 2, ParentID: 1, Name: "B"},
		}

		// Should not panic or hang, returns empty since no root (parentID=0)
		result := utils.BuildTree(elements,
			func(n treeNode) uint { return n.ParentID },
			func(n treeNode) uint { return n.ID },
			convert,
		)
		assert.Nil(t, result)
	})
}
