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
// @Summary      获取字典数据
// @Description  根据字典编码获取字典数据(用于下拉框，带缓存)
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        code path string true "字典编码"
// @Success      200 {object} response.Response "字典数据"
// @Router       /admin/v1/system/dict/data/{code} [get]
func (h *DictHandler) GetDictData(c *gin.Context) {
	dictCode := c.Param("code")
	if dictCode == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	data, err := h.dictService.ListData(c.Request.Context(), dictCode)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, data)
}

// ListType 字典类型列表
// @Summary      获取字典类型列表
// @Description  分页获取字典类型列表，支持按名称、编码、状态筛选
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        name query string false "字典类型名称"
// @Param        code query string false "字典类型编码"
// @Param        status query string false "状态(0/1)"
// @Success      200 {object} response.Response "字典类型列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/types [get]
func (h *DictHandler) ListType(c *gin.Context) {
	name := c.Query("name")
	code := c.Query("code")
	status := c.Query("status")
	page, err := strconv.Atoi(c.DefaultQuery("current", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("size", "20"))
	if err != nil || pageSize < 1 {
		pageSize = 20
	}

	list, total, err := h.dictService.ListType(c.Request.Context(), name, code, status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, page, pageSize, total, list)
}

// @Summary      创建字典类型
// @Description  新建一个字典类型
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        req body dict.CreateDictTypeReq true "创建字典类型参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/types [post]
func (h *DictHandler) CreateType(c *gin.Context) {
	var req dictDto.CreateDictTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	t := &dictEntity.DictType{
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		Description: req.Description,
	}

	if err := h.dictService.CreateType(c.Request.Context(), t); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      更新字典类型
// @Description  更新字典类型信息
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        req body dict.UpdateDictTypeReq true "更新字典类型参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/types [put]
func (h *DictHandler) UpdateType(c *gin.Context) {
	var req dictDto.UpdateDictTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
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
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      删除字典类型
// @Description  根据ID删除字典类型
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        id path int true "字典类型ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/types/{id} [delete]
func (h *DictHandler) DeleteType(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}
	if err := h.dictService.DeleteType(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// ListDataFull 字典数据全量管理列表
// @Summary      获取字典数据列表
// @Description  分页获取字典数据全量管理列表，支持按字典编码、标签、状态筛选
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        dictCode query string false "字典编码"
// @Param        label query string false "字典标签"
// @Param        status query string false "状态(0/1)"
// @Success      200 {object} response.Response "字典数据列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/data [get]
func (h *DictHandler) ListDataFull(c *gin.Context) {
	dictCode := c.Query("dictCode")
	label := c.Query("label")
	status := c.Query("status")
	page, err := strconv.Atoi(c.DefaultQuery("current", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("size", "20"))
	if err != nil || pageSize < 1 {
		pageSize = 20
	}

	list, total, err := h.dictService.ListDataFull(c.Request.Context(), dictCode, label, status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, page, pageSize, total, list)
}

// @Summary      创建字典数据
// @Description  新建一条字典数据
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        req body dict.CreateDictDataReq true "创建字典数据参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/data [post]
func (h *DictHandler) CreateData(c *gin.Context) {
	var req dictDto.CreateDictDataReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
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
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      更新字典数据
// @Description  更新字典数据信息
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        req body dict.UpdateDictDataReq true "更新字典数据参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/data [put]
func (h *DictHandler) UpdateData(c *gin.Context) {
	var req dictDto.UpdateDictDataReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
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
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      删除字典数据
// @Description  根据ID删除字典数据
// @Tags         字典管理
// @Accept       json
// @Produce      json
// @Param        id path int true "字典数据ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/dict/data/{id} [delete]
func (h *DictHandler) DeleteData(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}
	if err := h.dictService.DeleteData(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
