// RBAC 管理 API（V2 rbac——roles/menus/apis/buttons/授权绑定）。
import http from "@/api";

export interface RoleRow {
  id: string;
  code: string;
  name: string;
  status: number;
  remark?: string;
}

export interface MenuRow {
  id: string;
  parentId: string;
  name: string;
  path: string;
  component: string;
  icon: string;
  sortOrder: number;
  hidden: boolean;
  status: number;
  children?: MenuRow[];
}

export interface ButtonRow {
  id: string;
  menuId: string;
  code: string;
  name: string;
}

export interface ApiRow {
  id: string;
  method: string;
  path: string;
  act: string;
  summary?: string;
}

export const rbacApi = {
  roles: (page = 1, pageSize = 100) => http.page<RoleRow>("/admin/v1/roles", page, pageSize),
  createRole: (data: { code: string; name: string; remark?: string }) => http.post("/admin/v1/roles", data),
  updateRole: (id: string, data: { code: string; name?: string; remark?: string; status?: number }) =>
    http.put(`/admin/v1/roles/${id}`, data),
  deleteRole: (id: string) => http.delete(`/admin/v1/roles/${id}`),
  roleGrants: (id: string) => http.get<{ menus: string[]; apis: string[]; buttons: string[] }>(`/admin/v1/roles/${id}/grants`),
  setRoleMenus: (id: string, ids: string[]) => http.put(`/admin/v1/roles/${id}/menus`, { ids }),
  setRoleApis: (id: string, ids: string[]) => http.put(`/admin/v1/roles/${id}/apis`, { ids }),
  setRoleButtons: (id: string, ids: string[]) => http.put(`/admin/v1/roles/${id}/buttons`, { ids }),
  menuTree: () => http.get<MenuRow[]>("/admin/v1/route-tree"),
  menus: () => http.get<MenuRow[]>("/admin/v1/menus"),
  apis: (page = 1, pageSize = 200) => http.page<ApiRow>("/admin/v1/apis", page, pageSize),
  createApi: (data: { method: string; path: string; act: string; summary?: string }) => http.post("/admin/v1/apis", data),
  deleteApi: (id: string) => http.delete(`/admin/v1/apis/${id}`),
  buttons: () => http.get("/admin/v1/buttons"),
  createButton: (data: { menuId: string; code: string; name: string }) => http.post("/admin/v1/buttons", data),
  deleteButton: (id: string) => http.delete(`/admin/v1/buttons/${id}`),
  createMenu: (data: {
    parentId: string;
    name: string;
    path: string;
    component: string;
    icon: string;
    sortOrder: number;
    hidden: boolean;
    status: number;
  }) => http.post("/admin/v1/menus", data),
  updateMenu: (id: string, data: any) => http.put(`/admin/v1/menus/${id}`, data),
  deleteMenu: (id: string) => http.delete(`/admin/v1/menus/${id}`),
  routeTree: () => http.get("/admin/v1/route-tree")
};
