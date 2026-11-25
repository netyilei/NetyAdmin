// 任务中心 API（kernel scheduler 管理面——cmd/netyadmin/taskcenter.go 纯转发：
// jobs/runs 快照+记录，trigger/pause/resume 操作；信封由 axios 拦截器解包，
// 调用方拿信封对象后取 data。字段对照 scheduler.JobInfo / RunRecord）。
import http from "@/api";

export interface JobInfo {
  name: string;
  scope: string;
  trigger: string;
  schedule: string;
  timezone: string;
  paused: boolean;
  running: boolean;
}

export interface RunRecord {
  id: number;
  run_id: string;
  job: string;
  node: string;
  trigger_kind: string;
  shard: number;
  status: string; // running|succeeded|failed|unknown（runStatusText 出口归一）
  attempt: number;
  error: string;
  trace_id: string;
  started_at: string;
  finished_at?: string;
}

export const taskcenterApi = {
  jobs: () => http.get<{ jobs: JobInfo[] }>("/admin/v1/taskcenter/jobs"),
  runs: (name: string) => http.get<{ runs: RunRecord[] }>(`/admin/v1/taskcenter/jobs/${name}/runs`),
  trigger: (name: string) =>
    http.post<{ triggered: boolean; job: string; reason?: string }>(`/admin/v1/taskcenter/jobs/${name}/trigger`),
  pause: (name: string) => http.post<{ paused: boolean; job: string }>(`/admin/v1/taskcenter/jobs/${name}/pause`),
  resume: (name: string) => http.post<{ resumed: boolean; job: string }>(`/admin/v1/taskcenter/jobs/${name}/resume`)
};
