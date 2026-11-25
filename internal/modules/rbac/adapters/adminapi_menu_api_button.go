// adminapi_menu_api_button.go：菜单/API 登记/按钮管理端点（V1 menu 10 +
// api 6 + button 6 端点 RESTful 收敛——menus 5 + apis 5 + buttons 4；列表统一
// 全量/过滤返回，树组装归前端）。
package adapters

import (
	"github.com/gin-gonic/gin"

	// swag 注解解析需要（@Failure errorx.Envelope 类型解析——Go 编译面仅副作用无）
	_ "github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/rbac/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/rbac/domain"
)

// registerMenuRoutes 注册菜单域路由。
func registerMenuRoutes(rg gin.IRouter, deps Deps, secured securedChain) {
	g := rg.Group("/menus")
	g.GET("", secured(keys.PermMenusList, listMenusHandler(deps))...)
	g.POST("", secured(keys.PermMenusCrea, createMenuHandler(deps))...)
	g.GET("/:id", secured(keys.PermMenusGet, getMenuHandler(deps))...)
	g.PUT("/:id", secured(keys.PermMenusUpd, updateMenuHandler(deps))...)
	g.POST("/batch-delete", secured(keys.PermMenusDel, deleteMenusHandler(deps))...)
	g.DELETE("/:id", secured(keys.PermMenusDel, deleteMenuHandler(deps))...)
}

// @Summary 菜单全量列表（前端组树）
// @ID admin.rbac.menus.list
// @Tags menus
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/menus [get]
func listMenusHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		menus, err := deps.Service.ListMenus(c.Request.Context())
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, menus)
	}
}

// @Summary 菜单详情
// @ID admin.rbac.menus.get
// @Tags menus
// @Param id path string true "菜单 ID"
// @Success 200 {object} domain.Menu
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/menus/{id} [get]
func getMenuHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		m, err := deps.Service.GetMenu(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, m)
	}
}

type menuRequest struct {
	ParentID  string `json:"parentId" validate:"max=32"`
	Name      string `json:"name" validate:"required,max=64"`
	Path      string `json:"path" validate:"max=128"`
	SortOrder int    `json:"sortOrder" validate:"gte=0"`
	Hidden    bool   `json:"hidden"`
	Status    int    `json:"status" validate:"omitempty,eq=1|eq=2"`
}

// @Summary 创建菜单
// @ID admin.rbac.menus.create
// @Tags menus
// @Accept json
// @Param body body menuRequest true "菜单定义"
// @Success 200 {object} domain.Menu
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/menus [post]
func createMenuHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req menuRequest
		if !transport.Bind(c, &req) {
			return
		}
		m, err := deps.Service.CreateMenu(c.Request.Context(), menuFromReq(req))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, m)
	}
}

func updateMenuHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req menuRequest
		if !transport.Bind(c, &req) {
			return
		}
		m := menuFromReq(req)
		m.ID = c.Param("id")
		out, err := deps.Service.UpdateMenu(c.Request.Context(), m)
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, out)
	}
}

// @Summary 批量删除菜单
// @ID admin.rbac.menus.batch_delete
// @Description 级联清理 role_menus
// @Tags menus
// @Accept json
// @Param body body idsRequest true "菜单 ID 全集"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/menus/batch-delete [post]
func deleteMenusHandler(deps Deps) gin.HandlerFunc {
	return idsPut(deps, func(c *gin.Context, _ string, ids []string) error {
		return deps.Service.DeleteMenus(c.Request.Context(), ids)
	})
}

func menuFromReq(req menuRequest) domain.Menu {
	return domain.Menu{
		ParentID: req.ParentID, Name: req.Name, Path: req.Path,
		SortOrder: req.SortOrder, Hidden: req.Hidden, Status: domain.Status(req.Status),
	}
}

// registerApiRoutes 注册 API 登记域路由。
func registerApiRoutes(rg gin.IRouter, deps Deps, secured securedChain) {
	g := rg.Group("/apis")
	g.GET("", secured(keys.PermApisList, listApisHandler(deps))...)
	g.POST("", secured(keys.PermApisCrea, createApiHandler(deps))...)
	g.GET("/:id", secured(keys.PermApisGet, getApiHandler(deps))...)
	g.PUT("/:id", secured(keys.PermApisUpd, updateApiHandler(deps))...)
	g.DELETE("/:id", secured(keys.PermApisDel, deleteApiHandler(deps))...)
}

// @Summary API 登记全量列表
// @ID admin.rbac.apis.list
// @Tags apis
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/apis [get]
func listApisHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		apis, err := deps.Service.ListApis(c.Request.Context())
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, apis)
	}
}

// @Summary API 登记详情
// @ID admin.rbac.apis.get
// @Tags apis
// @Param id path string true "API ID"
// @Success 200 {object} domain.Api
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/apis/{id} [get]
func getApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		a, err := deps.Service.GetApi(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, a)
	}
}

type apiRequest struct {
	Method  string `json:"method" validate:"required,oneof=GET POST PUT PATCH DELETE *"`
	Path    string `json:"path" validate:"required,max=128"`
	Act     string `json:"act" validate:"required,max=64"`
	Summary string `json:"summary" validate:"max=128"`
	Status  int    `json:"status" validate:"omitempty,eq=1|eq=2"`
}

// @Summary 登记 API 判定行
// @ID admin.rbac.apis.create
// @Description act=权限码（与路由 RequirePerm 声明同值）；path=keyMatch2 模式
// @Tags apis
// @Accept json
// @Param body body apiRequest true "API 登记"
// @Success 200 {object} domain.Api
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/apis [post]
func createApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apiRequest
		if !transport.Bind(c, &req) {
			return
		}
		a, err := deps.Service.CreateApi(c.Request.Context(), apiFromReq(req))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, a)
	}
}

func updateApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apiRequest
		if !transport.Bind(c, &req) {
			return
		}
		a := apiFromReq(req)
		a.ID = c.Param("id")
		out, err := deps.Service.UpdateApi(c.Request.Context(), a)
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, out)
	}
}

// @Summary 删除 API 登记
// @ID admin.rbac.apis.delete
// @Description 级联清理 role_apis + 策略 reload
// @Tags apis
// @Param id path string true "API ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/apis/{id} [delete]
func deleteApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteApi(c.Request.Context(), c.Param("id")); err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

func apiFromReq(req apiRequest) domain.Api {
	return domain.Api{
		Method: req.Method, Path: req.Path, Act: req.Act,
		Summary: req.Summary, Status: domain.Status(req.Status),
	}
}

// registerButtonRoutes 注册按钮域路由。
func registerButtonRoutes(rg gin.IRouter, deps Deps, secured securedChain) {
	g := rg.Group("/buttons")
	g.GET("", secured(keys.PermButtonsList, listButtonsHandler(deps))...)
	g.POST("", secured(keys.PermButtonsCrea, createButtonHandler(deps))...)
	g.PUT("/:id", secured(keys.PermButtonsUpd, updateButtonHandler(deps))...)
	g.DELETE("/:id", secured(keys.PermButtonsDel, deleteButtonHandler(deps))...)
}

// @Summary 按钮列表（可按菜单过滤）
// @ID admin.rbac.buttons.list
// @Tags buttons
// @Param menu_id query string false "归属菜单过滤"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/buttons [get]
func listButtonsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		buttons, err := deps.Service.ListButtons(c.Request.Context(), c.Query("menu_id"))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, buttons)
	}
}

type buttonRequest struct {
	MenuID string `json:"menuId" validate:"max=32"`
	Code   string `json:"code" validate:"required,max=64"`
	Name   string `json:"name" validate:"required,max=64"`
}

// @Summary 创建按钮权限码
// @ID admin.rbac.buttons.create
// @Tags buttons
// @Accept json
// @Param body body buttonRequest true "按钮定义"
// @Success 200 {object} domain.Button
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/buttons [post]
func createButtonHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req buttonRequest
		if !transport.Bind(c, &req) {
			return
		}
		b, err := deps.Service.CreateButton(c.Request.Context(), domain.Button{
			MenuID: req.MenuID, Code: req.Code, Name: req.Name,
		})
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, b)
	}
}

func updateButtonHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req buttonRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.UpdateButton(c.Request.Context(), domain.Button{
			ID: c.Param("id"), MenuID: req.MenuID, Code: req.Code, Name: req.Name,
		}); err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary 删除按钮权限码
// @ID admin.rbac.buttons.delete
// @Description 级联清理 role_buttons
// @Tags buttons
// @Param id path string true "按钮 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/buttons/{id} [delete]
func deleteButtonHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteButton(c.Request.Context(), c.Param("id")); err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary 删除菜单（单条——内部走 batch-delete 同事务语义）
// @Tags menus
// @ID admin.rbac.menus.delete
// @Param id path string true "菜单 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/menus/{id} [delete]
func deleteMenuHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteMenus(c.Request.Context(), []string{c.Param("id")}); err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}
