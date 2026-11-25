// 系统配置 API（V2 configcenter——键值编辑+新增 upsert+回滚
// 装-A2 清偿）。PUT 契约=items 全量提交（对照 internal/modules/configcenter
// /adapters/adminapi.go updateItem——group/key/value/valueType(1字符串2整数
// 3布尔4JSON)/description/isPublic）。
import http from "@/api";

export interface ConfigRow {
  group: string;
  key: string;
  value: string;
  valueType: number;
  description?: string;
  isPublic?: boolean;
  updatedAt?: string;
}

export interface ConfigUpsertItem {
  group: string;
  key: string;
  value: string;
  valueType: number;
  description: string;
  isPublic: boolean;
}

export const configApi = {
  list: () => http.get<ConfigRow[]>("/admin/v1/configs"),
  update: (data: { items: ConfigUpsertItem[]; reason: string }) => http.put("/admin/v1/configs", data),
  del: (group: string, key: string) => http.delete(`/admin/v1/configs/${group}/${key}`),
  rollback: (version: number) => http.post("/admin/v1/configs/rollback", { version }),
  history: () => http.get("/admin/v1/configs/history")
};
