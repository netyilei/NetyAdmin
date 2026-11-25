// 通用格式化收口（此前 5 页各自复制 fmtTime + 2 处内联）
// ISO 时间戳 → "YYYY-MM-DD HH:mm:ss"；空值显示占位符。
export function fmtTime(t?: string | null, placeholder = "—"): string {
  return t ? t.slice(0, 19).replace("T", " ") : placeholder;
}
