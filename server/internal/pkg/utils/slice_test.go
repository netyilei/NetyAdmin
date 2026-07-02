package utils_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/pkg/utils"
)

func TestSliceMap(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		var nilSlice []int
		result := utils.SliceMap(nilSlice, func(x int) int { return x * 2 })
		assert.Nil(t, result)
	})

	t.Run("empty slice returns empty", func(t *testing.T) {
		result := utils.SliceMap([]int{}, func(x int) int { return x * 2 })
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("int to string conversion", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := utils.SliceMap(input, func(x int) string {
			return string(rune('A' + x - 1))
		})
		assert.Equal(t, []string{"A", "B", "C"}, result)
	})

	t.Run("int doubling", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		result := utils.SliceMap(input, func(x int) int { return x * 2 })
		assert.Equal(t, []int{2, 4, 6, 8, 10}, result)
	})

	t.Run("struct field extraction", func(t *testing.T) {
		type person struct {
			Name string
			Age  int
		}
		people := []person{
			{"Alice", 30},
			{"Bob", 25},
		}
		names := utils.SliceMap(people, func(p person) string { return p.Name })
		assert.Equal(t, []string{"Alice", "Bob"}, names)
	})

	t.Run("preserves length and order", func(t *testing.T) {
		input := []int{10, 20, 30}
		result := utils.SliceMap(input, func(x int) int { return x })
		assert.True(t, reflect.DeepEqual(input, result))
	})
}

func TestSliceSort(t *testing.T) {
	t.Run("sorts unsorted slice", func(t *testing.T) {
		items := []string{"banana", "apple", "cherry"}
		utils.SliceSort(items)
		assert.Equal(t, []string{"apple", "banana", "cherry"}, items)
	})

	t.Run("already sorted stays sorted", func(t *testing.T) {
		items := []string{"apple", "banana", "cherry"}
		utils.SliceSort(items)
		assert.True(t, sort.StringsAreSorted(items))
	})

	t.Run("empty slice", func(t *testing.T) {
		items := []string{}
		utils.SliceSort(items)
		assert.Len(t, items, 0)
	})

	t.Run("single element", func(t *testing.T) {
		items := []string{"only"}
		utils.SliceSort(items)
		assert.Equal(t, []string{"only"}, items)
	})

	t.Run("with duplicates", func(t *testing.T) {
		items := []string{"cherry", "apple", "banana", "apple"}
		utils.SliceSort(items)
		assert.Equal(t, []string{"apple", "apple", "banana", "cherry"}, items)
	})
}
