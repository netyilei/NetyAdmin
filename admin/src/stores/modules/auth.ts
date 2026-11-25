import { defineStore } from "pinia";
import { computed, ref } from "vue";

import { getAuthButtonListApi, getAuthMenuListApi } from "@/api/modules/login";
import { toMenuOptions } from "@/utils/routeTreeAdapter";
import { getAllBreadcrumbList, getFlatMenuList, getShowMenuList } from "@/utils";

export const useAuthStore = defineStore("geeker-auth", () => {
  // 按钮权限列表
  const authButtonList = ref<string[]>([]); // V2 扁平权限码集（admin.*——v-auth 消费）
  // 菜单权限列表
  const authMenuList = ref<Menu.MenuOptions[]>([]);
  // 当前页面的 router name，用来做按钮权限筛选
  const routeName = ref<string>("");

  // 按钮权限列表
  const authButtonListGet = computed(() => authButtonList.value);
  // 菜单权限列表 ==> 这里的菜单没有经过任何处理
  const authMenuListGet = computed(() => authMenuList.value);
  // 菜单权限列表 ==> 左侧菜单栏渲染，需要剔除 isHide == true
  const showMenuListGet = computed(() => getShowMenuList(authMenuList.value));
  // 菜单权限列表 ==> 扁平化之后的一维数组菜单，主要用来添加动态路由
  const flatMenuListGet = computed(() => getFlatMenuList(authMenuList.value));
  // 递归处理后的所有面包屑导航列表
  const breadcrumbListGet = computed(() => getAllBreadcrumbList(authMenuList.value));

  // Get AuthButtonList
  const getAuthButtonList = async () => {
    const { data } = await getAuthButtonListApi();
    authButtonList.value = data.acts ?? [];
  };

  // Get AuthMenuList
  const getAuthMenuList = async () => {
    const { data } = await getAuthMenuListApi();
    authMenuList.value = toMenuOptions(data ?? []);
  };

  // Set RouteName
  const setRouteName = async (name: string) => {
    routeName.value = name;
  };

  return {
    authButtonList,
    authMenuList,
    routeName,
    authButtonListGet,
    authMenuListGet,
    showMenuListGet,
    flatMenuListGet,
    breadcrumbListGet,
    getAuthButtonList,
    getAuthMenuList,
    setRouteName
  };
});
