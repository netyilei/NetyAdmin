import http from "@/api";
import { Login } from "@/api/interface/index";

/**
 * @description 用户登录（V2 identity——防枚举 10005；密码明文走 TLS，
 * 服务端 bcrypt——前端不散列）
 */
export const loginApi = (params: Login.ReqLoginForm) => {
  return http.post<Login.ResLogin>(`/admin/v1/auth/login`, params, { loading: false });
};

/**
 * @description 登录验证码（补强——英数 4 位一次性 3 分钟有效；
 * image=dataURL 直接展示）
 */
export const getCaptchaApi = () => {
  return http.get<{ captchaId: string; image: string }>(`/admin/v1/auth/captcha`, {}, { loading: false });
};

/**
 * @description 登出（V2：jti 吊销——可选；前端清 token 即失活）
 */
export const logoutApi = () => {
  return http.post(`/admin/v1/auth/logout`, {}, { loading: false });
};

/**
 * @description 获取菜单树（V2 RBAC route-tree——MenuNode 树；super-admin=
 * 全部启用菜单，否则 role_menus 并集）
 */
export const getAuthMenuListApi = () => {
  return http.get<Menu.MenuOptions[]>(`/admin/v1/route-tree`, {}, { loading: false });
};

/**
 * @description 获取权限码集（V2 rbac permissions——按钮 v-auth 消费）
 */
export const getAuthButtonListApi = () => {
  return http.get<Login.ResAuthButtons>(`/admin/v1/permissions`, {}, { loading: false });
};

/**
 * @description 管理员信息（V2 user-info——登录后回填 userStore）
 */
export const getUserInfoApi = () => {
  return http.get(`/admin/v1/auth/user-info`);
};
