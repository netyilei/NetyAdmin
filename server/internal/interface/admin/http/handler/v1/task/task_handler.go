package task

import (
	taskDto "NetyAdmin/internal/interface/admin/dto/task"
	"NetyAdmin/internal/pkg/errorx"
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

// ListTasks 获取任务列表
func (h *TaskHandler) ListTasks(c *gin.Context) {
	tasks, err := h.taskSvc.ListTasks(c.Request.Context())
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "获取任务列表失败")
		return
	}
	response.Success(c, tasks)
}

// RunTask 立即执行任务
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

// StartTask 启动任务
func (h *TaskHandler) StartTask(c *gin.Context) {
	name := c.Param("name")
	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	if err := h.taskSvc.StartTask(c.Request.Context(), name, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务已启动")
}

// StopTask 停止任务
func (h *TaskHandler) StopTask(c *gin.Context) {
	name := c.Param("name")
	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	if err := h.taskSvc.StopTask(c.Request.Context(), name, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务已停止")
}

// ReloadTask 重启任务
func (h *TaskHandler) ReloadTask(c *gin.Context) {
	name := c.Param("name")
	if err := h.taskSvc.ReloadTask(c.Request.Context(), name); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务已重启")
}

// UpdateTask 更新任务配置
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	var req taskDto.UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	if err := h.taskSvc.UpdateTask(c.Request.Context(), req.Name, req.Enabled, req.Spec, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, "任务配置已更新")
}

// ListLogs 获取任务执行日志
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
