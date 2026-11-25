// 动态路由适配：V2 RBAC route-tree（MenuNode 树）→ Geeker MenuOptions
// （component=视图路径 /src/views<component>.vue；icon/title/isHide 直映）。
// 首页 path 以 / 开头的完整路由；目录节点无 component=纯分组。

export function toMenuOptions(nodes: any[]): Menu.MenuOptions[] {
  return nodes.map(n => ({
    path: n.path,
    name: (n.path.replace(/\//g, "-").replace(/^-/, "") || n.id) + "-" + n.id.slice(-6),
    component: n.component || "",
    redirect: n.children && n.children.length ? n.children[0].path : undefined,
    meta: {
      icon: n.icon || "Menu",
      title: n.name,
      isLink: n.isLink || "",
      isHide: Boolean(n.hidden),
      isFull: false,
      isAffix: n.path === "/home/index",
      isKeepAlive: true
    },
    children: n.children && n.children.length ? toMenuOptions(n.children) : undefined
  }));
}
