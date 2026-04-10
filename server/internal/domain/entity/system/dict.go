package system

import "NetyAdmin/internal/domain/entity"

// DictType 字典类型实体
type DictType struct {
	entity.Model
	Name        string `gorm:"column:name;size:100;not null;comment:字典名称" json:"name"`
	Code        string `gorm:"column:code;size:100;not null;uniqueIndex:idx_dict_type_code,where:deleted_at=0;comment:字典编码" json:"code"`
	Status      string `gorm:"column:status;size:1;default:1;comment:状态(1启用 0禁用)" json:"status"`
	Description string `gorm:"column:description;size:200;comment:描述" json:"description"`
}

func (DictType) TableName() string {
	return "sys_dict_type"
}

// DictData 字典数据实体
type DictData struct {
	entity.Model
	DictCode string `gorm:"column:dict_code;size:100;not null;index:idx_dict_data_code;comment:字典编码" json:"dictCode"`
	Label    string `gorm:"column:label;size:100;not null;comment:字典标签" json:"label"`
	Value    string `gorm:"column:value;size:100;not null;comment:字典键值" json:"value"`
	TagType  string `gorm:"column:tag_type;size:20;default:default;comment:标签类型(success|info|warning|error|default)" json:"tagType"`
	OrderBy  int    `gorm:"column:order_by;default:0;comment:排序" json:"orderBy"`
	Status   string `gorm:"column:status;size:1;default:1;comment:状态(1启用 0禁用)" json:"status"`
	Remark   string `gorm:"column:remark;size:200;comment:备注" json:"remark"`
}

func (DictData) TableName() string {
	return "sys_dict_data"
}
