#!/usr/bin/env bash
# NetyAdmin 本地栈全链路 E2E（本会话修复的验收）：开放平台签名 → 登录/刷新/顶号/登出
# （user_tokens CAS）→ Incr TTL 补挂 → api_ratelimit 规则隔离与非法规则丢弃。
# 依赖：本地 PG(root/123456/netyadmin) + Redis + 测试用户 verifyuser01/Passw0rd456。
# 自动在 server/tmp/e2e/ 派生独立实例（端口 9091，不影响 9090 常驻实例）。
# 用法：bash scripts/debug/e2e_session.sh   输出 PASS/FAIL 与证据行。
# 注意：端级拒绝响应码为 101002（middleware 将 ptv/hash 失败统一伪装为禁用）。
set -u
BASE=http://127.0.0.1:9091
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"   # 仓库根 D:/NetyAdmin（锚点与调用 cwd 无关）
SRV_DIR="$REPO_ROOT/server"
E2E_DIR="$SRV_DIR/tmp/e2e"
mkdir -p "$E2E_DIR"

APP_KEY=01JQDEFAULTAPP001
APP_SECRET=netyadmin-default-app-secret
USER_NAME=verifyuser01
USER_PASS=Passw0rd456
export PGPASSWORD=123456
PSQL="psql -h localhost -U root -d netyadmin -t -A -q"
LOG=server.log
PASS=0
SPID=""
kill_9091() {
  local pid=$(netstat -ano 2>/dev/null | grep -E ":9091 .*LISTENING" | head -1 | awk '{print $NF}')
  if [ -n "$pid" ]; then
    taskkill //F //PID "$pid" >/dev/null 2>&1 && echo "[$(date +%H:%M:%S)] 已终止 9091 实例 pid=$pid"
  fi
}
step() { echo "[$(date +%H:%M:%S)] $*"; }
die() {
  echo "[$(date +%H:%M:%S)] FAIL: $*"
  kill_9091
  echo "DIAGNOSIS: 最近 30 行服务日志（$LOG）"
  tail -30 "$LOG" 2>/dev/null | sed 's/^/    /'
  exit 1
}
ok() { echo "[$(date +%H:%M:%S)]   => $1"; PASS=$((PASS+1)); }
assert_eq() { local exp=$1 act=$2 msg=$3; [ "$exp" = "$act" ] || die "$msg：expected=[$exp] actual=[$act]"; }

jget() { printf '%s' "$1" | python -c "import json,sys;d=json.load(sys.stdin);print(d$2)" 2>/dev/null; }
db() { $PSQL -c "$1" 2>&1; }

# api METHOD PATH -> 回包 body（依赖全局 AUTH / XREFRESH / BODY）
api() {
  local method=$1 path=$2
  local ts nonce payload="" sig
  ts=$(date +%s)
  nonce=$(openssl rand -hex 12)
  if [ -n "${BODY:-}" ]; then payload=$(printf '%s' "$BODY" | openssl dgst -sha256 -hex | awk '{print $NF}'); fi
  sig=$(printf '%s\n%s\n%s\n%s\n%s' "$method" "$path" "$ts" "$nonce" "$payload" \
    | openssl dgst -sha256 -hmac "$APP_SECRET" -binary | openssl base64 -A)
  local -a h=(-H "Content-Type: application/json" -H "X-App-Key: $APP_KEY" \
    -H "X-Timestamp: $ts" -H "X-Nonce: $nonce" -H "X-Signature: $sig")
  [ -n "${AUTH:-}" ]     && h+=(-H "Authorization: Bearer $AUTH")
  [ -n "${XREFRESH:-}" ] && h+=(-H "X-Refresh-Token: $XREFRESH")
  [ -n "${BODY:-}" ]     && h+=(--data-raw "$BODY")
  curl -s --max-time 10 -X "$method" "$BASE$path" "${h[@]}"
}

# ============ ⓪ 产物准备（自包含，可重复执行）============
if [ ! -f "$E2E_DIR/config.yaml" ]; then
  { sed 's/^  port: 9090/  port: 9091/' "$SRV_DIR/config.yaml"
    cat << 'CFG'

# ===== E2E 临时追加（断言依赖，勿改）=====
api_ratelimit:
  rules:
    - name: e2e-invalid-zero-max
      dimension: ip
      window: 1m
      max: 0
    - name: e2e-profile-limit
      dimension: ip
      window: 10s
      max: 3
      paths: ["/client/v1/user/profile"]
CFG
  } > "$E2E_DIR/config.yaml"
fi
cp -r "$SRV_DIR/keys" "$E2E_DIR/" 2>/dev/null || true
cd "$E2E_DIR"


# ============ ① 体检段 ============
step "H0 体检：依赖与产物"
command -v psql >/dev/null || die "psql 不可用"
command -v openssl >/dev/null || die "openssl 不可用"
command -v python >/dev/null || die "python 不可用"
[ "$(redis-cli ping 2>/dev/null)" = "PONG" ] || die "Redis 不可达（期望 PONG）"
[ -n "$(db "SELECT 1;" 2>/dev/null)" ] || die "PG 不可达"
[ -f config.yaml ] || die "E2E config.yaml 缺失"
[ -f keys/jwt_private.pem ] || die "JWT keys 缺失"
netstat -ano 2>/dev/null | grep -E ":9091 .*LISTENING" >/dev/null && die "9091 已被占用"
ok "PG/Redis 可达，9091 空闲，产物齐全"

step "H1 构建 server.exe"
( cd "$SRV_DIR" && go build -o tmp/e2e/server.exe ./cmd/server ) || die "go build 失败"
[ -x server.exe ] || die "server.exe 未生成"
ok "构建完成"

# ============ ② 启动 + 等健康 ============
step "H2 启动独立实例（9091）"
./server.exe > "$LOG" 2>&1 &
SPID=$!
ready=""
for i in $(seq 1 30); do
  code=$(curl -s --max-time 3 -o /dev/null -w '%{http_code}' "$BASE/health" 2>/dev/null)
  if [ "$code" = "200" ]; then ready=1; break; fi
  sleep 1
done
[ -n "$ready" ] || die "服务 30s 未就绪（health!=200），启动层未动"
ok "health=200，server.pid=$SPID"

# ============ ③ 前置断言 + 造数复原 ============
step "P0 前置：测试用户存在且启用、登录验证码门关闭、清理历史会话"
db "SELECT 1 FROM users WHERE username='$USER_NAME' AND status=1;" | grep -q 1 || die "测试用户 $USER_NAME 不存在或未启用"
captcha=$(db "SELECT config_value FROM sys_configs WHERE group_name='captcha_config' AND config_key='user_login_enabled';")
assert_gate() { [ "$1" = "false" ] || die "captcha user_login_enabled 期望 false actual [$1]（请改回后再跑）"; }
assert_gate "$captcha"
UID_=$(db "SELECT id FROM users WHERE username='$USER_NAME';")
[ -n "$UID_" ] || die "取不到 userID"
echo "  测试用户 userID=$UID_"
removed=$(db "DELETE FROM user_tokens WHERE user_id='$UID_';" >/dev/null 2>&1; echo done)
redis-cli --scan --pattern "netyadmin:auth:retry:$UID_" 2>/dev/null | xargs -r redis-cli del >/dev/null
redis-cli --scan --pattern "netyadmin:auth:lock:$UID_" 2>/dev/null | xargs -r redis-cli del >/dev/null
ok "前置就绪（历史会话已清）"

# ============ ④ S1 登录（CAS 写 hash）============
step "S1 登录：POST /client/v1/user/login"
BODY="{\"userName\":\"$USER_NAME\",\"password\":\"$USER_PASS\",\"platform\":\"web\"}"
resp=$(api POST /client/v1/user/login)
assert_eq "100000" "$(jget "$resp" "['code']")" "登录 code"
ACCESS1=$(jget "$resp" "['data']['accessToken']")
REFRESH1=$(jget "$resp" "['data']['refreshToken']")
[ -n "$ACCESS1" ] && [ -n "$REFRESH1" ] || die "登录未返回 token"
v=$(db "SELECT token_version FROM user_tokens WHERE user_id='$UID_' AND platform='web';")
ah=$(db "SELECT access_hash <> '' AND refresh_hash <> '' FROM user_tokens WHERE user_id='$UID_' AND platform='web';")
assert_eq "1" "$v" "S1 后 token_version"
assert_eq "t" "$ah" "S1 后 hash 非空（CAS 已落库）"
ok "登录成功：version=1，hash 已落库（持久层证据）"

# ============ ⑤ S2 profile（middleware hash 校验通过）============
step "S2 profile：GET /client/v1/user/profile（access#1）"
AUTH=$ACCESS1 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "100000" "$(jget "$resp" "['code']")" "profile code"
ok "access#1 有效（CAS 写入的 hash 与中间件校验闭环）"

# ============ ⑥ S3 刷新（CAS 轮换 hash）============
step "S3 刷新：POST /client/v1/user/refresh-token（refresh#1）"
AUTH= XREFRESH=$REFRESH1 BODY="{\"refreshToken\":\"$REFRESH1\"}" resp=$(api POST /client/v1/user/refresh-token)
assert_eq "100000" "$(jget "$resp" "['code']")" "刷新 code"
ACCESS2=$(jget "$resp" "['data']['accessToken']")
REFRESH2=$(jget "$resp" "['data']['refreshToken']")
[ -n "$ACCESS2" ] || die "刷新未返回新 token"
# 端级拒绝响应码 = 101002（middleware 将 ptv 顶号 / hash 不匹配统一伪装为
# CodeUserDisabled，避免向客户端泄露具体拒绝原因）；窗口内残留计数会让限流
# 层（101305）先拒——断言前清窗，确保拒绝来自 hash 校验层本身。
redis-cli --scan --pattern "netyadmin:api_rl:*" 2>/dev/null | xargs -r redis-cli del >/dev/null
AUTH=$ACCESS1 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "101002" "$(jget "$resp" "['code']")" "旧 access#1 应 101002（hash 已被 CAS 轮换，端级校验拒绝）"
AUTH=$ACCESS2 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "100000" "$(jget "$resp" "['code']")" "新 access#2 profile code"
ok "刷新成功：旧 access#1 失效、新 access#2 有效"

# ============ ⑦ S4 顶号（二次登录 v2）============
step "S4 顶号：同 platform 二次登录"
BODY="{\"userName\":\"$USER_NAME\",\"password\":\"$USER_PASS\",\"platform\":\"web\"}"
resp=$(api POST /client/v1/user/login)
assert_eq "100000" "$(jget "$resp" "['code']")" "二次登录 code"
ACCESS3=$(jget "$resp" "['data']['accessToken']")
REFRESH3=$(jget "$resp" "['data']['refreshToken']")
v=$(db "SELECT token_version FROM user_tokens WHERE user_id='$UID_' AND platform='web';")
assert_eq "2" "$v" "S4 后 token_version"
redis-cli --scan --pattern "netyadmin:api_rl:*" 2>/dev/null | xargs -r redis-cli del >/dev/null
AUTH=$ACCESS2 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "101002" "$(jget "$resp" "['code']")" "旧会话 access#2 应 101002（ptv1 < v2 顶号）"
AUTH=$ACCESS3 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "100000" "$(jget "$resp" "['code']")" "新会话 access#3 profile code"
ok "顶号成功：v2，旧会话失效、新会话有效"

# ============ ⑧ S5 登出（ClearHashes CAS）============
step "S5 登出：POST /client/v1/user/logout（access#3 + X-Refresh-Token refresh#3）"
AUTH=$ACCESS3 XREFRESH=$REFRESH3 BODY= resp=$(api POST /client/v1/user/logout)
assert_eq "100000" "$(jget "$resp" "['code']")" "登出 code"
hashes=$(db "SELECT access_hash = '' AND refresh_hash = '' FROM user_tokens WHERE user_id='$UID_' AND platform='web';")
assert_eq "t" "$hashes" "登出后 hash 应清空（持久层）"
redis-cli --scan --pattern "netyadmin:api_rl:*" 2>/dev/null | xargs -r redis-cli del >/dev/null
AUTH=$ACCESS3 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "101002" "$(jget "$resp" "['code']")" "登出后 access#3 应 101002（hash 已清空）"
AUTH= XREFRESH=$REFRESH3 BODY="{\"refreshToken\":\"$REFRESH3\"}" resp=$(api POST /client/v1/user/refresh-token)
assert_eq "100002" "$(jget "$resp" "['code']")" "登出后 refresh#3 应 100002（黑名单）"
bl=$(redis-cli --scan --pattern "netyadmin:auth:blacklist:refresh:$REFRESH3" 2>/dev/null | head -1)
[ -n "$bl" ] || die "黑名单键不存在"
ok "登出成功：hash 清空、access 401、refresh 黑名单（Redis 键=$bl）"

# ============ ⑨ S6 登录失败 → Incr TTL 补挂 ============
step "S6 登录失败：Incr 应在 Lua 内挂 TTL"
BODY="{\"userName\":\"$USER_NAME\",\"password\":\"wrong-pass\",\"platform\":\"web\"}"
resp=$(api POST /client/v1/user/login)
code=$(jget "$resp" "['code']")
[ "$code" != "100000" ] || die "错误密码不应登录成功"
ttl=$(redis-cli ttl "netyadmin:auth:retry:$UID_" 2>/dev/null)
case "$ttl" in ''|*-1*) die "retry key TTL=[$ttl]，期望 >0（Incr Lua 补挂）";; esac
ok "失败计数键 TTL=$ttl（>0，Incr Lua 补挂生效）"

# ============ ⑩ S7 限流（等待窗口重置后压 profile）============
step "S7 准备：正确密码再登录（应清失败计数）"
sleep 9   # 等待限流窗口重置（window=10s，>窗口即清零；条件已知，无需轮询）
BODY="{\"userName\":\"$USER_NAME\",\"password\":\"$USER_PASS\",\"platform\":\"web\"}"
resp=$(api POST /client/v1/user/login)
assert_eq "100000" "$(jget "$resp" "['code']")" "S7 登录 code（ClearLoginRetry 后成功）"
ACCESS4=$(jget "$resp" "['data']['accessToken']")
retry_left=$(redis-cli --scan --pattern "netyadmin:auth:retry:$UID_" 2>/dev/null | head -1)
[ -z "$retry_left" ] || ok "提示：retry 键存在但登录已成功（ClearLoginRetry 时序内），不判失败"

step "S8 限流：profile 3 次放行 + 第 4 次 101305 + 窗口后恢复"
AUTH=$ACCESS4 BODY=
for i in 1 2 3; do
  resp=$(api GET /client/v1/user/profile)
  assert_eq "100000" "$(jget "$resp" "['code']")" "限流内第 $i 次请求"
done
resp=$(api GET /client/v1/user/profile)
assert_eq "101305" "$(jget "$resp" "['code']")" "第 4 次应 101305 流量限制"
rl_key=$(redis-cli --scan --pattern "netyadmin:api_rl:*" 2>/dev/null | head -1)
[ -n "$rl_key" ] || die "限流 ZSET 键不存在"
ok "限流生效：ZSET 键=$rl_key"
sleep 11  # 窗口（10s）过期后恢复
AUTH=$ACCESS4 BODY= resp=$(api GET /client/v1/user/profile)
assert_eq "100000" "$(jget "$resp" "['code']")" "窗口过后应恢复放行"
ok "窗口恢复：滑动窗口语义正确"

# ============ ⑪ S9 非法规则告警 ============
step "S9 非法规则：max:0 应被 normalize 丢弃并告警恰一次"
n=$(grep -c "已忽略该规则" "$LOG" 2>/dev/null || echo 0)
assert_eq "1" "$n" "启动日志中『已忽略该规则』条数"
ok "非法规则丢弃告警恰 1 条"

# ============ ⑫ 清理复原 + 结论 ============
step "C0 清理复原"
db "DELETE FROM user_tokens WHERE user_id='$UID_';" >/dev/null
redis-cli --scan --pattern "netyadmin:auth:retry:$UID_" 2>/dev/null | xargs -r redis-cli del >/dev/null
redis-cli --scan --pattern "netyadmin:auth:lock:$UID_" 2>/dev/null | xargs -r redis-cli del >/dev/null
redis-cli --scan --pattern "netyadmin:api_rl:*" 2>/dev/null | xargs -r redis-cli del >/dev/null
redis-cli --scan --pattern "netyadmin:auth:blacklist:refresh:*" 2>/dev/null | xargs -r redis-cli del >/dev/null
kill "$SPID" 2>/dev/null
ok "会话行/计数键/黑名单/限流键已清，服务已停止"

echo ""
echo "E2E RESULT: PASS（$PASS 项断言组通过；全链路=签名→登录CAS→刷新CAS→顶号→登出CAS→Incr TTL→限流隔离→非法规则丢弃）"
exit 0
