package task

type UpdateTaskReq struct {
	Name    string `json:"name" binding:"required"`
	Enabled bool   `json:"enabled"`
	Spec    string `json:"spec" binding:"required"`
}
