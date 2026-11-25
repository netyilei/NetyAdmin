import type { ProxyOptions } from "vite";

type ProxyItem = [string, string];

type ProxyList = ProxyItem[];

type ProxyTargetList = Record<string, ProxyOptions>;

/**
 * 创建代理，用于解析 .env.development 代理配置
 * @param list
 */
export function createProxy(list: ProxyList = []) {
  const ret: ProxyTargetList = {};
  for (const [prefix, target] of list) {
    const httpsRE = /^https:\/\//;
    const isHttps = httpsRE.test(target);

    // https://github.com/http-party/node-http-proxy#options
    ret[prefix] = {
      target: target,
      changeOrigin: true,
      ws: true,
      // V2 后端路由带完整前缀（/admin/v1/...）——原样转发不剥前缀
      // （Geeker 默认剥前缀适配 mock 场景；基座适配）
      // https is require secure=false
      ...(isHttps ? { secure: false } : {})
    };
  }
  return ret;
}
