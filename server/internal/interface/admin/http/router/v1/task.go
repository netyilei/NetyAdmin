package v1

import (
	"github.com/gin-gonic/gin"

	taskHandler "NetyAdmin/internal/interface/admin/http/handler/v1/task"
)

type TaskRouter struct {
	task *taskHandler.TaskHandler
}

func NewTaskRouter(task *taskHandler.TaskHandler) *TaskRouter {
	return &TaskRouter{task: task}
}

func (r *TaskRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *TaskRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *TaskRouter) RegisterPermission(group *gin.RouterGroup) {
	task := group.Group("/system/tasks")
	{
		task.GET("", r.task.ListTasks)
		task.POST("/:name/run", r.task.RunTask)
		task.POST("/:name/start", r.task.StartTask)
		task.POST("/:name/stop", r.task.StopTask)
		task.POST("/:name/reload", r.task.ReloadTask)
		task.PUT("/update", r.task.UpdateTask)
		task.GET("/logs", r.task.ListLogs)
	}
}
