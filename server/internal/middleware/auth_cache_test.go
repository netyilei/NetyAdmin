package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	systemRepoPkg "NetyAdmin/internal/repository/system"
)

// TestFetchLoaderReturnTypeAssignment 验证 LazyCacheManager.assign 的赋值兼容性：
// Fetch 的 loader 返回 (*AdminAuthState, error)，dest 是 *systemRepoPkg.AdminAuthState。
// assign 实现：marshal(loader 返回值) → unmarshal 到 dest。
//
// 关键风险点：loader 返回的 *AdminAuthState 经 json.Marshal 后，
// 再 unmarshal 到 *systemRepoPkg.AdminAuthState（dest 必须是指针的指针或同类型指针）。
// 此测试用同包内 assign 等价逻辑模拟，确认类型可正确赋值。
func TestFetchLoaderReturnTypeAssignment(t *testing.T) {
	// 模拟 loader 返回值
	state := &systemRepoPkg.AdminAuthState{
		TokenVersion: 42,
		Status:       "1",
	}

	// 模拟 assign：marshal loader 返回值，再 unmarshal 到 dest（*AdminAuthState）
	b, err := json.Marshal(state)
	require.NoError(t, err)

	var dest *systemRepoPkg.AdminAuthState
	err = json.Unmarshal(b, &dest)
	require.NoError(t, err)

	// 验证赋值成功，dest 指向新对象且字段正确
	require.NotNil(t, dest)
	assert.Equal(t, uint64(42), dest.TokenVersion)
	assert.Equal(t, "1", dest.Status)
}

// TestAdminAuthStateEmptyValue 验证零值 AdminAuthState 经 marshal/unmarshal 后的语义。
// 风险点：token_version=0 + status="" 是合法的零值，不应被误判为 nil 或缓存穿透。
func TestAdminAuthStateEmptyValue(t *testing.T) {
	state := &systemRepoPkg.AdminAuthState{}

	b, err := json.Marshal(state)
	require.NoError(t, err)

	// 零值序列化后应包含字段名（非 null）
	assert.Contains(t, string(b), "TokenVersion")
	assert.Contains(t, string(b), "Status")

	var dest *systemRepoPkg.AdminAuthState
	err = json.Unmarshal(b, &dest)
	require.NoError(t, err)
	require.NotNil(t, dest)
	assert.Equal(t, uint64(0), dest.TokenVersion)
	assert.Equal(t, "", dest.Status)
}

// TestLoaderReturnsNilPointer 验证 loader 返回 nil 指针时 assign 行为。
// 模拟 adminRepo.GetAuthStateByID 在记录不存在时返回 (nil, err) 的场景。
// Fetch 实现中 isNil() 会拦截 nil 指针，不写缓存，直接 assign(nil, dest)。
func TestLoaderReturnsNilPointer(t *testing.T) {
	var nilState *systemRepoPkg.AdminAuthState

	b, err := json.Marshal(nilState)
	require.NoError(t, err)

	// nil 指针序列化为 "null"
	assert.Equal(t, "null", string(b))

	var dest *systemRepoPkg.AdminAuthState
	err = json.Unmarshal(b, &dest)
	require.NoError(t, err)
	// unmarshal "null" 后 dest 仍是 nil
	assert.Nil(t, dest)
}
