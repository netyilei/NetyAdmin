// 管理员管理 API（V2 identity——信封由 axios 拦截器解包，
// 调用方拿信封对象后取 data；分页 Page{list,total,page,page_size}）。
import http from "@/api";

export interface AdminRow {
  id: string;
  username: string;
  name: string;
  email: string; // 列表响应已掩码（PII）；原文经 get 详情（admin.admins.get 权限）
  phone?: string; // 同上（列表掩码）
  status: number;
  mustChangePassword: boolean;
  lastLoginAt: string | null;
  createdAt: string;
}

export const adminApi = {
  list: (page: number, pageSize: number, query?: Record<string, string | undefined>) =>
    http.page<AdminRow>("/admin/v1/admins", page, pageSize, query),
  // 详情（admin.admins.get 权限）：编辑表单取 email/phone 原文的唯一入口
  get: (id: string) => http.get<AdminRow>(`/admin/v1/admins/${id}`),
  create: (data: { username: string; password: string; name?: string; email?: string }) => http.post("/admin/v1/admins", data),
  update: (id: string, data: { name?: string; email?: string }) => http.put(`/admin/v1/admins/${id}`, data),
  delete: (id: string) => http.delete(`/admin/v1/admins/${id}`),
  batchDelete: (ids: string[]) => http.post("/admin/v1/admins/batch-delete", { ids }),
  setStatus: (id: string, status: number) => http.patch(`/admin/v1/admins/${id}/status`, { status }),
  resetPassword: (id: string) => http.post<{ password: string }>(`/admin/v1/admins/${id}/reset-password`),
  roles: (id: string) => http.get<{ roles: string[] }>(`/admin/v1/admins/${id}/roles`),
  setRoles: (id: string, roleIds: string[]) => http.put(`/admin/v1/admins/${id}/roles`, { ids: roleIds }),
  changePassword: (data: { old_password: string; new_password: string }) => http.post("/admin/v1/auth/change-password", data),
  userInfo: () => http.get("/admin/v1/auth/user-info"),
  updateProfile: (data: { name: string; email: string; phone: string; avatarUrl: string }) =>
    http.put("/admin/v1/auth/profile", data)
};
