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
