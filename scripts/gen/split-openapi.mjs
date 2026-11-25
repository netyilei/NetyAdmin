#!/usr/bin/env node
// split-openapi.mjs：swag 单次生成物 → 按接入面切分 contracts/openapi/{surface}.v1.yaml。
//
// 背景：swag 一次 init 会把全部注解收进一个 spec（三面路径混排），
// 而对外契约按面分文件（破坏式变更各自升版本）。本脚本按 path 前缀切分并把
// components.schemas 按 $ref 闭包各自内嵌——三文件 paths 集合互斥、引用完整，
// 生成物直接入库（禁手改）。
//
// 用法：
//   1) swag init --v3.1 --parseDependency --parseInternal -g cmd/netyadmin/main.go -o <tmp> --ot yaml
//   2) node scripts/gen/split-openapi.mjs <tmp>/swagger.yaml
//
// 切分规则：/admin/v1/* → admin.v1.yaml；/client/v1/* → client.v1.yaml；
// /gateway/v1/* → gateway.v1.yaml。未匹配任何前缀的 path = 注解落了新面 → 报错退出。
//
// 安全面注入：
//   - securitySchemes 随面内嵌：admin/client=BearerAuth（swag @securityDefinitions.apikey
//     生成）；gateway=GatewayHMAC（swag v2 单 definition 限制——在脚本注入四头锚定定义）；
//   - 每面根 security 默认要求对应 scheme；publicPaths 逐 operation 注入 `security: []`
//     （登录/验证码/刷新/公开配置/WS upgrade 等免鉴权面），避免「全有或全无」失实。
import fs from "node:fs";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..", "..");
const OUT_DIR = path.join(ROOT, "contracts", "openapi");
const SRC = process.argv[2];

if (!SRC) {
  console.error("用法: node scripts/gen/split-openapi.mjs <swag 生成的 swagger.yaml>");
  process.exit(1);
}

const SURFACES = [
  {
    name: "admin",
    prefix: "/admin/v1/",
    server: "/admin/v1",
    info: null, // 复用生成物 info（@title/@version/@description 来自 main.go）
    scheme: "BearerAuth",
    // 免鉴权面（实测挂载：identity public 组 + configcenter public 组）
    publicPaths: [
      "/admin/v1/auth/login",
      "/admin/v1/auth/captcha",
      "/admin/v1/auth/refresh-token",
      "/admin/v1/configs/public",
    ],
  },
  {
    name: "client",
    prefix: "/client/v1/",
    server: "/client/v1",
    info: [
      "info:",
      "  description: C 端 API 契约（client 面——clientapi 用户/站内信/存储；swag init --v3.1 按 surface 切分生成）",
      "  title: NetyAdmin Client API",
      "  version: 1.0.0",
    ],
    scheme: "BearerAuth",
    // 免鉴权面：注册/登录；WS upgrade=ticket query（ticket 由 authed /ws/tickets 换发）
    publicPaths: ["/client/v1/users/login", "/client/v1/users/register", "/client/v1/ws"],
  },
  {
    name: "gateway",
    prefix: "/gateway/v1/",
    server: "/gateway/v1",
    info: [
      "info:",
      "  description: 网关 API 契约（gateway 面——edge HMAC 管线/WS ticket 与业务转发；swag init --v3.1 按 surface 切分生成）",
      "  title: NetyAdmin Gateway API",
      "  version: 1.0.0",
    ],
    scheme: "GatewayHMAC",
    // WS upgrade=ticket query 免 HMAC；ticket 签发与业务转发均过 edge HMAC 管线
    publicPaths: ["/gateway/v1/ws"],
  },
];

const text = fs.readFileSync(SRC, "utf8").replace(/\r\n/g, "\n");
const lines = text.split("\n");

// —— 顶层分区（swag 输出为固定缩进：顶层键 0 缩进、path 键 2、schema 名 4；键名含大写如 externalDocs）——
const sectionStarts = [];
lines.forEach((l, i) => {
  const m = /^([A-Za-z][A-Za-z0-9_]*):/.exec(l);
  if (m) sectionStarts.push({ key: m[1], line: i });
});
const sectionOf = (key) => {
  const idx = sectionStarts.findIndex((s) => s.key === key);
  if (idx < 0) return null;
  const end = idx + 1 < sectionStarts.length ? sectionStarts[idx + 1].line : lines.length;
  return lines.slice(sectionStarts[idx].line, end);
};

const pathsSection = sectionOf("paths");
const componentsSection = sectionOf("components");
if (!pathsSection || !componentsSection) {
  console.error(`[FAIL] 生成物缺 paths/components 顶层段: ${SRC}`);
  process.exit(1);
}

// paths: 每个 "  /xxx:" 起一条，直到下一个 path 键
const pathEntries = new Map();
let curPath = null;
for (const l of pathsSection.slice(1)) {
  const m = /^ {2}(\/.*):$/.exec(l);
  if (m) {
    curPath = m[1];
    pathEntries.set(curPath, [l]);
  } else if (curPath !== null) {
    pathEntries.get(curPath).push(l);
  }
}

// components.schemas: 每个 "    <name>:" 起一条（直到下一个 schema 或 components 子段）
const schemaEntries = new Map();
let curSchema = null;
let inSchemas = false;
for (const l of componentsSection.slice(1)) {
  if (/^ {2}schemas:$/.test(l)) {
    inSchemas = true;
    continue;
  }
  if (!inSchemas) continue;
  if (/^ {2}\S/.test(l)) {
    inSchemas = false; // components 其它子段
    continue;
  }
  const m = /^ {4}(\S.*):$/.exec(l);
  if (m) {
    curSchema = m[1];
    schemaEntries.set(curSchema, [l]);
  } else if (curSchema !== null) {
    schemaEntries.get(curSchema).push(l);
  }
}

if (pathEntries.size === 0) {
  console.error("[FAIL] 生成物 paths 为空");
  process.exit(1);
}

// components.securitySchemes：每个 "    <name>:" 起一条（swag @securityDefinitions.apikey）
const securitySchemeEntries = new Map();
let curScheme = null;
let inSecuritySchemes = false;
for (const l of componentsSection.slice(1)) {
  if (/^ {2}securitySchemes:$/.test(l)) {
    inSecuritySchemes = true;
    continue;
  }
  if (!inSecuritySchemes) continue;
  if (/^ {2}\S/.test(l)) {
    inSecuritySchemes = false; // components 其它子段
    continue;
  }
  const m = /^ {4}(\S.*):$/.exec(l);
  if (m) {
    curScheme = m[1];
    securitySchemeEntries.set(curScheme, [l]);
  } else if (curScheme !== null) {
    securitySchemeEntries.get(curScheme).push(l);
  }
}

// GatewayHMAC 定义（swag v2 生成物只保留单一 securityDefinitions 块——网关面
// 在脚本侧注入；四头语义见 docs/guide/gateway-canonical.md）。
const GATEWAY_HMAC_SCHEME = [
  "    GatewayHMAC:",
  "      description: 开放平台 HMAC 四头：X-NA-App-Id / X-NA-Timestamp / X-NA-Nonce / X-NA-Signature（本定义锚定签名头；canonical 见 docs/guide/gateway-canonical.md）",
  "      in: header",
  "      name: X-NA-Signature",
  "      type: apiKey",
];

// injectEmptySecurity 给 path chunk 内每个 operation 追加 `security: []`
//（免鉴权面覆盖根级 security；OpenAPI 3.1 security 只允许 operation/root 级）。
function injectEmptySecurity(chunk) {
  const isMethod = (l) => /^ {4}(get|post|put|patch|delete):$/.test(l);
  const out = [];
  for (let i = 0; i < chunk.length; i++) {
    out.push(chunk[i]);
    if (!isMethod(chunk[i])) continue;
    let j = i + 1;
    while (j < chunk.length && !isMethod(chunk[j])) j++;
    const body = chunk.slice(i + 1, j);
    let end = body.length;
    while (end > 0 && body[end - 1] === "") end--;
    out.push(...body.slice(0, end), "      security: []", ...body.slice(end));
    i = j - 1;
  }
  return out;
}

const refsIn = (chunk) =>
  [...chunk.join("\n").matchAll(/\$ref: '#\/components\/schemas\/([^']+)'/g)].map((m) => m[1]);

// —— 每个 path 必须恰好归属一个面（防新面注解悄悄混入 admin）——
const assigned = new Map();
for (const p of pathEntries.keys()) {
  const hits = SURFACES.filter((s) => p.startsWith(s.prefix));
  if (hits.length !== 1) {
    console.error(`[FAIL] path ${p} 未匹配唯一接入面前缀（命中 ${hits.length} 个）——为新增前缀补 SURFACES 规则`);
    process.exit(1);
  }
  assigned.set(p, hits[0].name);
}

const externalDocs = sectionOf("externalDocs");
// swag 恒输出空 externalDocs（description/url 为 "" 占位）——空块不落盘
// （oasdiff validate 会报 external-docs-url-required）；有内容（@externalDocs 注解）才保留。
const externalDocsUseful =
  externalDocs &&
  externalDocs.some((l) => {
    const m = /^\s+(?:description|url):\s*(.*)$/.exec(l);
    const v = m ? m[1].trim() : "";
    return v !== "" && v !== '""';
  })
    ? externalDocs
    : null;
const openapiLine = sectionOf("openapi");

for (const surf of SURFACES) {
  const paths = [...pathEntries.keys()].filter((p) => assigned.get(p) === surf.name);
  if (paths.length === 0) {
    console.error(`[FAIL] ${surf.name} 面 paths 为空——检查注解与前缀规则`);
    process.exit(1);
  }

  // $ref 闭包（组件可引用组件——迭代到不动点）
  const needed = new Set();
  const queue = paths.flatMap((p) => refsIn(pathEntries.get(p)));
  while (queue.length > 0) {
    const name = queue.pop();
    if (needed.has(name)) continue;
    const chunk = schemaEntries.get(name);
    if (!chunk) {
      console.error(`[FAIL] ${surf.name} 面引用缺失 schema: ${name}`);
      process.exit(1);
    }
    needed.add(name);
    queue.push(...refsIn(chunk));
  }

  const out = ["components:", "  schemas:"];
  // 保持生成物原始顺序（swag 按名字排序），仅过滤
  for (const [name, chunk] of schemaEntries) {
    if (needed.has(name)) out.push(...chunk);
  }
  // securitySchemes：面内只保留本面使用的 scheme（admin/client=BearerAuth，
  // gateway=GatewayHMAC 脚本注入）
  const schemeLines = surf.scheme === "GatewayHMAC"
    ? GATEWAY_HMAC_SCHEME
    : securitySchemeEntries.get(surf.scheme);
  if (!schemeLines) {
    console.error(`[FAIL] ${surf.name} 面缺 securityScheme ${surf.scheme}（swag @securityDefinitions 注解丢失？）`);
    process.exit(1);
  }
  out.push("  securitySchemes:", ...schemeLines);
  if (externalDocsUseful) out.push(...externalDocsUseful);
  if (surf.info) out.push(...surf.info);
  else out.push(...sectionOf("info"));
  if (openapiLine) out.push(...openapiLine);
  // 根级安全声明（面默认要求对应 scheme）；免鉴权 operation 已注入 security: []
  out.push("security:", `- ${surf.scheme}: []`);
  out.push("paths:");
  for (const p of paths) {
    const chunk = pathEntries.get(p);
    out.push(...(surf.publicPaths.includes(p) ? injectEmptySecurity(chunk) : chunk));
  }
  out.push("servers:", `- url: ${surf.server}`);

  // 去掉分块拼接产生的结尾空行（顶层段之间保持单空行边界由各段自身携带）
  while (out.length > 0 && out[out.length - 1] === "") out.pop();
  const file = path.join(OUT_DIR, `${surf.name}.v1.yaml`);
  fs.writeFileSync(file, out.join("\n") + "\n");
  console.log(`  ${surf.name}.v1.yaml: ${paths.length} paths, ${needed.size} schemas`);
}

console.log(`[OK] split ${pathEntries.size} paths (${SRC}) → ${OUT_DIR}`);
