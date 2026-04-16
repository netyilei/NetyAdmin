# 字典模块详解

本文档详细介绍 NetyAdmin 字典模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

字典模块提供动态字典管理能力，支持字典类型和字典数据的CRUD操作，用于前端渲染下拉选择、状态标签等场景。

### 1.1 核心特性

- **动态配置**：后台实时增删改字典数据
- **缓存加速**：字典数据自动缓存，提高读取性能
- **类型隔离**：不同类型字典独立管理
- **启用控制**：支持单独启用/禁用字典项

---

## 二、目录结构

```
server/internal/domain/entity/system/
├── dict.go             # 字典实体定义

server/internal/repository/system/
├── dict.go             # 字典仓储

server/internal/service/system/
├── dict.go             # 字典服务

server/internal/interface/admin/http/handler/v1/system/
├── dict_handler.go     # 字典Handler
```

---

## 三、数据模型

### 3.1 字典类型（sys_dict_type）

```go
type SysDictType struct {
    ID          uint           `gorm:"primarykey"`
    Code        string         `gorm:"size:64;not null;uniqueIndex"`  // 字典编码（唯一标识）
    Name        string         `gorm:"size:128;not null"`             // 字典名称
    Description string         `gorm:"size:256"`                      // 描述
    Status      int8           `gorm:"default:1"`                     // 状态：1启用 2禁用
    CreatedAt   int64          `gorm:"autoCreateTime"`
    UpdatedAt   int64          `gorm:"autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}
```

### 3.2 字典数据（sys_dict_data）

```go
type SysDictData struct {
    ID         uint           `gorm:"primarykey"`
    TypeCode   string         `gorm:"size:64;not null;index"`        // 所属字典类型编码
    Label      string         `gorm:"size:128;not null"`             // 显示标签
    Value      string         `gorm:"size:128;not null"`             // 字典值
    Sort       int            `gorm:"default:0"`                     // 排序
    Status     int8           `gorm:"default:1"`                     // 状态：1启用 2禁用
    Remark     string         `gorm:"size:256"`                      // 备注
    CreatedAt  int64          `gorm:"autoCreateTime"`
    UpdatedAt  int64          `gorm:"autoUpdateTime"`
    DeletedAt  gorm.DeletedAt `gorm:"index"`
}
```

---

## 四、API接口

### 4.1 字典类型管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/system/dict/types | 获取字典类型列表 |
| POST | /admin/v1/system/dict/types | 创建字典类型 |
| PUT | /admin/v1/system/dict/types | 更新字典类型 |
| DELETE | /admin/v1/system/dict/types/:id | 删除字典类型 |

### 4.2 字典数据管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/system/dict/data | 获取字典数据列表 |
| POST | /admin/v1/system/dict/data | 创建字典数据 |
| PUT | /admin/v1/system/dict/data | 更新字典数据 |
| DELETE | /admin/v1/system/dict/data/:id | 删除字典数据 |
| GET | /admin/v1/system/dict/data/:code | 按编码获取启用的字典数据（前端用） |

---

## 五、使用示例

### 5.1 后端使用

```go
// 获取字典数据
func (s *dictService) GetDictDataByCode(ctx context.Context, code string) ([]*entity.SysDictData, error) {
    // 先查缓存
    cacheKey := cache.KeyDictData(code)
    if cached, err := s.cacheManager.Get(ctx, cacheKey); err == nil {
        return cached.([]*entity.SysDictData), nil
    }
    
    // 回源查询
    data, err := s.dictRepo.GetDataByTypeCode(ctx, code)
    if err != nil {
        return nil, err
    }
    
    // 写入缓存
    s.cacheManager.Set(ctx, cacheKey, data, 30*time.Minute)
    
    return data, nil
}

// 清除缓存（数据变更时）
func (s *dictService) ClearDictCache(ctx context.Context, typeCode string) {
    s.cacheManager.Delete(ctx, cache.KeyDictData(typeCode))
    s.cacheManager.DeleteByTag(ctx, cache.TagDict)
}
```

### 5.2 前端使用

```typescript
// 获取字典数据
const { data } = await fetchGetDictData('article_status')

// 渲染下拉选择
<NSelect :options="data.map(item => ({ label: item.label, value: item.value }))" />

// 渲染状态标签
const statusMap = {
  '1': { label: '已发布', type: 'success' },
  '2': { label: '草稿', type: 'warning' },
  '3': { label: '已下架', type: 'error' }
}
```

---

## 六、二次开发示例

### 6.1 新增字典类型

通过管理后台操作：

1. 进入【系统设置】->【字典管理】
2. 点击【新增类型】
3. 填写信息：
   - 编码：`order_status`
   - 名称：`订单状态`
   - 描述：`订单的各种状态`

### 6.2 新增字典数据

```sql
-- 插入订单状态字典数据
INSERT INTO sys_dict_data (type_code, label, value, sort, status) VALUES
('order_status', '待支付', 'pending', 1, 1),
('order_status', '已支付', 'paid', 2, 1),
('order_status', '已发货', 'shipped', 3, 1),
('order_status', '已完成', 'completed', 4, 1),
('order_status', '已取消', 'cancelled', 5, 1);
```

### 6.3 在代码中使用字典

```go
// 订单服务中使用字典
func (s *orderService) GetOrderStatusText(ctx context.Context, statusCode string) string {
    dictData, err := s.dictService.GetDictDataByCode(ctx, "order_status")
    if err != nil {
        return statusCode
    }
    
    for _, item := range dictData {
        if item.Value == statusCode {
            return item.Label
        }
    }
    
    return statusCode
}
```

---

## 七、缓存策略

### 7.1 缓存Key

```
dict:type:{code}      # 字典类型信息
dict:data:{code}      # 字典数据列表
```

### 7.2 缓存失效

| 操作 | 失效策略 |
|------|----------|
| 修改字典类型 | 清除该类型缓存 |
| 修改字典数据 | 清除所属类型数据缓存 |
| 删除字典 | 清除相关所有缓存 |

---

## 八、最佳实践

1. **编码规范**：字典编码使用小写字母+下划线，如 `article_status`
2. **值设计**：字典值建议使用字符串，便于扩展
3. **排序利用**：使用sort字段控制前端显示顺序
4. **缓存利用**：频繁读取的字典数据充分利用缓存
5. **禁用而非删除**：历史数据引用的字典项建议禁用而非删除

---

## 九、相关文档

- [Server架构设计](./server-architecture.md)
- [缓存模块详解](./server-module-cache.md)
