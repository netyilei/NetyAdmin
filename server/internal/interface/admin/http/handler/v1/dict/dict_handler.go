package dict

import (
	"strconv"

	dictEntity "NetyAdmin/internal/domain/entity/dict"
	dictDto "NetyAdmin/internal/interface/admin/dto/dict"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	dictSvc "NetyAdmin/internal/service/dict"

	"github.com/gin-gonic/gin"
)

type DictHandler struct {
	dictService dictSvc.DictService
}

func NewDictHandler(dictService dictSvc.DictService) *DictHandler {
	return &DictHandler{dictService: dictService}
}

// GetDictData 获取特定类型的字典数据(用于下拉框，带缓存)
func (h *DictHandler) GetDictData(c *gin.Context) {
	dictCode := c.Param("code")
	if dictCode == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	data, err := h.dictService.ListData(c.Request.Context(), dictCode)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}

	response.Success(c, data)
}

// ListType 字典类型列表
func (h *DictHandler) ListType(c *gin.Context) {
	name := c.Query("name")
	code := c.Query("code")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	list, total, err := h.dictService.ListType(c.Request.Context(), name, code, status, page, pageSize)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}

	response.SuccessWithPage(c, page, pageSize, total, list)
}

func (h *DictHandler) CreateType(c *gin.Context) {
	var req dictDto.CreateDictTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, err.Error())
		return
	}

	t := &dictEntity.DictType{
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		Description: req.Description,
	}

	if err := h.dictService.CreateType(c.Request.Context(), t); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *DictHandler) UpdateType(c *gin.Context) {
	var req dictDto.UpdateDictTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, err.Error())
		return
	}

	t := &dictEntity.DictType{
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		Description: req.Description,
	}
	t.ID = req.ID

	if err := h.dictService.UpdateType(c.Request.Context(), t); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *DictHandler) DeleteType(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.dictService.DeleteType(c.Request.Context(), uint(id)); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}

// ListDataFull 字典数据全量管理列表
func (h *DictHandler) ListDataFull(c *gin.Context) {
	dictCode := c.Query("dictCode")
	label := c.Query("label")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	list, total, err := h.dictService.ListDataFull(c.Request.Context(), dictCode, label, status, page, pageSize)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}

	response.SuccessWithPage(c, page, pageSize, total, list)
}

func (h *DictHandler) CreateData(c *gin.Context) {
	var req dictDto.CreateDictDataReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, err.Error())
		return
	}

	d := &dictEntity.DictData{
		DictCode: req.DictCode,
		Label:    req.Label,
		Value:    req.Value,
		TagType:  req.TagType,
		OrderBy:  req.OrderBy,
		Status:   req.Status,
		Remark:   req.Remark,
	}

	if err := h.dictService.CreateData(c.Request.Context(), d); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *DictHandler) UpdateData(c *gin.Context) {
	var req dictDto.UpdateDictDataReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, err.Error())
		return
	}

	d := &dictEntity.DictData{
		DictCode: req.DictCode,
		Label:    req.Label,
		Value:    req.Value,
		TagType:  req.TagType,
		OrderBy:  req.OrderBy,
		Status:   req.Status,
		Remark:   req.Remark,
	}
	d.ID = req.ID

	if err := h.dictService.UpdateData(c.Request.Context(), d); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *DictHandler) DeleteData(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.dictService.DeleteData(c.Request.Context(), uint(id)); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}
