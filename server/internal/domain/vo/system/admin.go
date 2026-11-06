package system

type LoginVO struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

type AdminInfoVO struct {
	UserID   string   `json:"userId"`
	Username string   `json:"userName"`
	Roles    []string `json:"roles"`
	Buttons  []string `json:"buttons"`
}

type ProfileVO struct {
	ID        uint   `json:"id"`
	Username  string `json:"userName"`
	Nickname  string `json:"nickName"`
	Phone     string `json:"userPhone"`
	Email     string `json:"userEmail"`
	Gender    string `json:"userGender"`
	Status    string `json:"status"`
	CreatedAt string `json:"createTime"`
}

// AdminDetailVO 是管理端「管理员详细资料」视图：与列表（掩码）互补，
// PII 为完整值（详情按权限查看）。
type AdminDetailVO struct {
	ID        uint     `json:"id"`
	Username  string   `json:"userName"`
	Nickname  string   `json:"nickName"`
	Phone     string   `json:"userPhone"`
	Email     string   `json:"userEmail"`
	Gender    *string  `json:"userGender"`
	Status    string   `json:"status"`
	Roles     []string `json:"userRoles"`
	Creator   string   `json:"createBy"`
	CreatedAt string   `json:"createTime"`
	Updater   string   `json:"updateBy"`
	UpdatedAt string   `json:"updateTime"`
}

type AdminItemVO struct {
	ID        uint     `json:"id"`
	Username  string   `json:"userName"`
	Nickname  string   `json:"nickName"`
	Phone     string   `json:"userPhone"`
	Email     string   `json:"userEmail"`
	Gender    *string  `json:"userGender"`
	Status    string   `json:"status"`
	Roles     []string `json:"userRoles"`
	Creator   string   `json:"createBy"`
	CreatedAt string   `json:"createTime"`
	Updater   string   `json:"updateBy"`
	UpdatedAt string   `json:"updateTime"`
}
