import { defineStore } from "pinia";
import { ref, type UnwrapRef } from "vue";

import { DEFAULT_PRIMARY } from "@/config";
import type { AssemblySizeType, LanguageType, LayoutType } from "@/stores/interface";

export const useGlobalStore = defineStore(
  "geeker-global",
  () => {
    // 布局模式 (纵向：vertical | 横向：transverse | 分栏：columns——classic 已删 2026-11-21)
    const layout = ref<LayoutType>("vertical");
    // element 组件大小
    const assemblySize = ref<AssemblySizeType>("default");
    // 当前系统语言
    const language = ref<LanguageType>(null);
    // 当前页面是否全屏
    const maximize = ref<boolean>(false);
    // 主题颜色
    const primary = ref<string>(DEFAULT_PRIMARY);
    // 深色模式
    const isDark = ref<boolean>(false);
    // 灰色模式
    const isGrey = ref<boolean>(false);
    // 色弱模式
    const isWeak = ref<boolean>(false);
    // 侧边栏反转
    const asideInverted = ref<boolean>(false);
    // 头部反转
    const headerInverted = ref<boolean>(false);
    // 折叠菜单
    const isCollapse = ref<boolean>(false);
    // 菜单手风琴
    const accordion = ref<boolean>(true);
    // 页面水印
    const watermark = ref<boolean>(false);
    // 面包屑导航
    const breadcrumb = ref<boolean>(true);
    // 面包屑导航图标
    const breadcrumbIcon = ref<boolean>(true);
    // 标签页
    const tabs = ref<boolean>(true);
    // 标签页图标
    const tabsIcon = ref<boolean>(true);
    // 页脚
    const footer = ref<boolean>(true);

    const stateMap = {
      layout,
      assemblySize,
      language,
      maximize,
      primary,
      isDark,
      isGrey,
      isWeak,
      asideInverted,
      headerInverted,
      isCollapse,
      accordion,
      watermark,
      breadcrumb,
      breadcrumbIcon,
      tabs,
      tabsIcon,
      footer
    } as const;

    // Set GlobalState
    const setGlobalState = <K extends keyof typeof stateMap>(key: K, value: UnwrapRef<(typeof stateMap)[K]>) => {
      const stateRef = stateMap[key];
      if (stateRef) {
        stateRef.value = value;
      }
    };

    return {
      ...stateMap,
      setGlobalState
    };
  },
  {
    persist: {
      key: "geeker-global",
      storage: localStorage
    }
  }
);
