package system

import (
	systemDto "netyadmin/internal/interface/admin/dto/system"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
	systemService "netyadmin/internal/service/system"
	"strconv"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	taskSvc systemService.TaskService
}

func NewTaskHandler(taskSvc systemService.TaskService) *TaskHandler {
	return &TaskHandler{taskSvc: taskSvc}
}

// @Summary      获取任务列表
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks [get]
func (h *TaskHandler) ListTasks(c *gin.Context) {
	tasks, err := h.taskSvc.ListTasks(c.Request.Context())
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "获取任务列表失败")
		return
	}
	response.Success(c, tasks)
}

// @Summary      立即执行任务
// @Param        name path string true "任务名"
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks/:name/run [post]
func (h *TaskHandler) RunTask(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "任务名称不能为空")
		return
	}

	if err := h.taskSvc.RunTask(c.Request.Context(), name); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "触发任务失败")
		return
	}

	response.Success(c, "任务触发成功")
}

// @Summary      启动任务
// @Param        name path string true "任务名"
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks/:name/start [post]
func (h *TaskHandler) StartTask(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("userID")
	operatorID := userID.(uint)

	if err := h.taskSvc.StartTask(c.Request.Context(), name, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务已启动")
}

// @Summary      停止任务
// @Param        name path string true "任务名"
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks/:name/stop [post]
func (h *TaskHandler) StopTask(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("userID")
	operatorID := userID.(uint)

	if err := h.taskSvc.StopTask(c.Request.Context(), name, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务已停止")
}

// @Summary      重启任务
// @Param        name path string true "任务名"
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks/:name/reload [post]
func (h *TaskHandler) ReloadTask(c *gin.Context) {
	name := c.Param("name")
	if err := h.taskSvc.ReloadTask(c.Request.Context(), name); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务已重启")
}

// @Summary      更新任务配置
// @Param        body body systemDto.UpdateTaskReq true "任务配置"
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks/:name [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	var req systemDto.UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	userID, _ := c.Get("userID")
	operatorID := userID.(uint)

	if err := h.taskSvc.UpdateTask(c.Request.Context(), req.Name, req.Enabled, req.Spec, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务配置已更新")
}

// @Summary      获取任务执行日志
// @Param        name query string false "任务名"
// @Param        page query int false "页码"
// @Param        size query int false "每页条数"
// @Tags         任务调度管理
// @Router       /admin/v1/system/tasks/logs [get]
func (h *TaskHandler) ListLogs(c *gin.Context) {
	name := c.Query("name")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	logs, total, err := h.taskSvc.ListLogs(c.Request.Context(), name, page, size)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "获取任务日志失败")
		return
	}

	response.Success(c, gin.H{
		"list":  logs,
		"total": total,
	})
}
