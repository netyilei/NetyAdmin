// 网关治理 API（..04 admin 面——apps CRUD/重置密钥/scope 授权
// + stats 统计 + IP 访问控制组/规则（配置组模型）；路径/字段对照
// contracts/openapi/admin.v1.yaml 与 internal/modules/gw/adapters/adminapi.go。
// 信封由拦截器解包，调用方取 data）。
import http from "@/api";

export interface GwAppRow {
  id: string;
  appKey: string;
  name: string;
  status: number; // 1=启用 2=禁用（gw_domain.Status）
  rateLimitEnabled: boolean;
  quota: Record<string, unknown> | null;
  remark: string;
  storageRef: string;
  ipGroupIds: string[]; // IP 策略组集
  scopes: string[];
  createdAt: string;
  updatedAt: string;
}

export interface GwSecretResult {
  appKey: string;
  secret: string;
}

// scope 组清单（available-scopes——后端 domain.ScopeGroup 出口 camelCase json tag）。
export interface GwScopeGroup {
  id: string;
  code: string;
  name: string;
  description: string;
  status: number; // 1=启用 2=停用（停用组不可被新授权）
  createdAt: string;
  updatedAt: string;
}

// stats overview 四指标（——Overview{hours,total,errors,errorRate,avgLatency,p95,apps,apis}）。
export interface GwStatsOverview {
  hours: number;
  total: number;
  errors: number;
  errorRate: number;
  avgLatency: number;
  p95: number;
  apps: number;
  apis: number;
}

export const gwApi = {
  // —— apps ——
  apps: (page: number, pageSize: number) => http.page<GwAppRow>("/admin/v1/open/apps", page, pageSize),
  createApp: (data: {
    name: string;
    appKey: string;
    secret: string;
    remark?: string;
    rateLimitEnabled?: boolean;
    quota?: { rate: number };
    ipGroupIds?: string[];
    scopes?: string[];
  }) => http.post("/admin/v1/open/apps", data),
  updateApp: (
    id: string,
    data: {
      name?: string;
      remark?: string;
      rateLimitEnabled?: boolean;
      quota?: { rate: number };
      ipGroupIds?: string[];
      scopes?: string[];
    }
  ) => http.put(`/admin/v1/open/apps/${id}`, data),
  deleteApp: (id: string) => http.delete(`/admin/v1/open/apps/${id}`),
  // 启停（PUT /open/apps/:id/status——adminapi.go 注册，权限码=admin.gw.apps.update；
  // body {status:1|2}，后端 validate eq=1|eq=2）
  setAppStatus: (id: string, status: number) => http.put(`/admin/v1/open/apps/${id}/status`, { status }),
  resetSecret: (id: string) => http.put<GwSecretResult>(`/admin/v1/open/apps/${id}/reset-secret`),
  // —— scope 组清单（应用表单多选数据源） ——
  availableScopes: () => http.get<GwScopeGroup[]>("/admin/v1/open/apps/available-scopes"),
  // —— stats（查询面读内存+快照文件零 DB） ——
  statsOverview: (hours?: number) => http.get<GwStatsOverview>("/admin/v1/open/stats/overview", { hours }),
  statsTrend: (hours?: number) => http.get("/admin/v1/open/stats/trend", { hours }),
  statsTop: (kind: string, limit?: number, hours?: number) => http.get("/admin/v1/open/stats/top", { kind, limit, hours }),
  statsStatus: (hours?: number) => http.get("/admin/v1/open/stats/status", { hours }),
  statsLatency: (hours?: number) => http.get("/admin/v1/open/stats/latency", { hours })
};

// —— IP 访问控制（配置组——组/规则管理；gw 引擎组集严格合成；应用在
// 应用平台页绑定组集。后端变更自动 Reload+Bus 广播全节点） ——
// IPGroup/IPRule 后端有 camelCase json tag（domain/ipac.go）——接口收敛必填。
export interface IPGroup {
  id: string;
  name: string;
  remark: string;
  createdAt: string;
  updatedAt: string;
}

export interface IPRule {
  id: string;
  groupId: string;
  cidrs: string;
  action: number; // 1=allow(白名单) 2=deny(黑名单)
  priority: number;
  enabled: boolean;
  remark: string;
  createdAt: string;
  updatedAt: string;
}

export const ipacApi = {
  groups: () => http.get<IPGroup[]>("/admin/v1/open/ip-groups"),
  createGroup: (data: { name: string; remark?: string }) => http.post("/admin/v1/open/ip-groups", data),
  updateGroup: (id: string, data: { name: string; remark?: string }) => http.put(`/admin/v1/open/ip-groups/${id}`, data),
  deleteGroup: (id: string) => http.delete(`/admin/v1/open/ip-groups/${id}`),
  rules: (groupId: string) => http.get<IPRule[]>(`/admin/v1/open/ip-rules?groupId=${groupId}`),
  createRule: (data: { groupId: string; cidrs: string; action: number; priority: number; enabled: boolean; remark?: string }) =>
    http.post("/admin/v1/open/ip-rules", data),
  deleteRule: (id: string) => http.delete(`/admin/v1/open/ip-rules/${id}`),
  reload: () => http.post("/admin/v1/open/ip-reload")
};

// —— 接口权限页（scope 组字典 + 网关 API 登记 + scope→API 白名单） ——
// ScopeGroupRow/GwApiRow：后端 domain 已出 camelCase json tag——单一形态。
export interface ScopeGroupRow {
  id: string;
  code: string;
  name: string;
  description: string;
  status: number; // 1=启用 2=停用（停用组不可被新授权）
  createdAt: string;
  updatedAt: string;
}

export interface GwApiRow {
  id: string;
  method: string;
  path: string;
  name: string;
  grp: string;
  description: string;
  status: number;
  createdAt: string;
  updatedAt: string;
}

export const scopeApi = {
  // —— scope 组 ——
  groups: () => http.get<ScopeGroupRow[]>("/admin/v1/open/scope-groups"),
  createGroup: (data: { code: string; name: string; description?: string; status?: number }) =>
    http.post("/admin/v1/open/scope-groups", data),
  // 编辑必传 code（后端 scopeGroupRequest required——：漏传触发 10002
  // 编辑必败；code 为组唯一键，编辑态不可改）
  updateGroup: (id: string, data: { code: string; name: string; description?: string; status: number }) =>
    http.put(`/admin/v1/open/scope-groups/${id}`, data),
  deleteGroup: (id: string) => http.delete(`/admin/v1/open/scope-groups/${id}`),
  // —— 网关 API 登记 ——
  apis: () => http.get<GwApiRow[]>("/admin/v1/open/apis"),
  createApi: (data: { method: string; path: string; name: string; grp?: string; description?: string; status?: number }) =>
    http.post("/admin/v1/open/apis", data),
  updateApi: (
    id: string,
    data: { method: string; path: string; name: string; grp?: string; description?: string; status?: number }
  ) => http.put(`/admin/v1/open/apis/${id}`, data),
  deleteApi: (id: string) => http.delete(`/admin/v1/open/apis/${id}`),
  // —— scope→API 白名单（scope=组 code；apiIDs 全量替换） ——
  scopeApis: (scope: string) => http.get<GwApiRow[]>(`/admin/v1/open/apis/scope-apis?scope=${scope}`),
  setScopeApis: (scope: string, apiIDs: string[]) => http.put("/admin/v1/open/apis/scope-apis", { scope, apis: apiIDs })
};
