package task

import (
	taskDto "NetyAdmin/internal/interface/admin/dto/task"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	taskSvc "NetyAdmin/internal/service/task"
	"strconv"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	taskSvc taskSvc.TaskService
}

func NewTaskHandler(taskSvc taskSvc.TaskService) *TaskHandler {
	return &TaskHandler{taskSvc: taskSvc}
}

// @Summary      获取任务列表
// @Description  获取所有定时任务列表
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "任务列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks [get]
func (h *TaskHandler) ListTasks(c *gin.Context) {
	tasks, err := h.taskSvc.ListTasks(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tasks)
}

// @Summary      立即执行任务
// @Description  根据任务名称立即触发执行一次
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Param        name path string true "任务名称"
// @Success      200 {object} response.Response "触发成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks/{name}/run [post]
func (h *TaskHandler) RunTask(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "任务名称不能为空")
		return
	}

	if err := h.taskSvc.RunTask(c.Request.Context(), name); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, "任务触发成功")
}

// @Summary      启动任务
// @Description  根据任务名称启动定时任务
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Param        name path string true "任务名称"
// @Success      200 {object} response.Response "启动成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks/{name}/start [post]
func (h *TaskHandler) StartTask(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "任务名称不能为空")
		return
	}
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	if err := h.taskSvc.StartTask(c.Request.Context(), name, operatorID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, "任务已启动")
}

// @Summary      停止任务
// @Description  根据任务名称停止定时任务
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Param        name path string true "任务名称"
// @Success      200 {object} response.Response "停止成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks/{name}/stop [post]
func (h *TaskHandler) StopTask(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "任务名称不能为空")
		return
	}
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	if err := h.taskSvc.StopTask(c.Request.Context(), name, operatorID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, "任务已停止")
}

// @Summary      重启任务
// @Description  根据任务名称重新加载任务配置
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Param        name path string true "任务名称"
// @Success      200 {object} response.Response "重启成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks/{name}/reload [post]
func (h *TaskHandler) ReloadTask(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "任务名称不能为空")
		return
	}
	if err := h.taskSvc.ReloadTask(c.Request.Context(), name); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, "任务已重启")
}

// @Summary      更新任务配置
// @Description  更新定时任务的启用状态与调度表达式
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Param        req body taskDto.UpdateTaskReq true "更新任务请求"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks/update [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	var req taskDto.UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	if err := h.taskSvc.UpdateTask(c.Request.Context(), req.Name, req.Enabled, req.Spec, operatorID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, "任务配置已更新")
}

// @Summary      获取任务执行日志
// @Description  分页获取任务执行日志
// @Tags         任务管理
// @Accept       json
// @Produce      json
// @Param        name query string false "任务名称"
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "日志列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/system/tasks/logs [get]
func (h *TaskHandler) ListLogs(c *gin.Context) {
	name := c.Query("name")
	page, err := strconv.Atoi(c.DefaultQuery("current", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "20"))
	if err != nil || size < 1 {
		size = 20
	}
	page, size = pagination.NormalizePagination(page, size)

	logs, total, err := h.taskSvc.ListLogs(c.Request.Context(), name, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, page, size, total, logs)
}
