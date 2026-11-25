// model.go：默认 Casbin 模型（RBAC 角色 + keyMatch2 路径匹配 + act 精确/通配）。
// v3 语法（examples/rbac_model.conf + keymatch2_model.conf 口径）。
// subject = admin 角色（token claims 携带）；obj = 路由/资源路径；
// act = 权限点（如 admin.users.list）或 HTTP method，支持 "*" 通配。
package authz

// defaultModel 是 RBAC 判定的默认模型文本（可用 Options.ModelText 覆盖——
// 下游 scope 域模型等场景）。
const defaultModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`
