import { defineStore } from "pinia";
import { ref } from "vue";

import router from "@/routers";
import type { TabsMenuProps } from "@/stores/interface";
import { getUrlWithParams } from "@/utils";

import { useKeepAliveStore } from "./keepAlive";

export const useTabsStore = defineStore(
  "geeker-tabs",
  () => {
    const keepAliveStore = useKeepAliveStore();

    const tabsMenuList = ref<TabsMenuProps[]>([]);

    // Add Tabs
    const addTabs = async (tabItem: TabsMenuProps) => {
      if (tabsMenuList.value.every(item => item.path !== tabItem.path)) {
        tabsMenuList.value.push(tabItem);
      }
      // add keepalive
      // 缓存口径=fullPath：tabItem.path 由 Tabs watcher 传入 route.fullPath
      // （含 query），与 Main/index.vue wrapper 名（route.fullPath）同源命中；
      // 字段名 path 是历史误导，勿改回 route.path 口径（带 query 页将永不命中）。
      if (!keepAliveStore.keepAliveName.includes(tabItem.name) && tabItem.isKeepAlive) {
        keepAliveStore.addKeepAliveName(tabItem.path);
      }
    };

    // Remove Tabs
    const removeTabs = async (tabPath: string, isCurrent: boolean = true) => {
      if (isCurrent) {
        tabsMenuList.value.forEach((item, index) => {
          if (item.path !== tabPath) return;
          const nextTab = tabsMenuList.value[index + 1] || tabsMenuList.value[index - 1];
          if (!nextTab) return;
          router.push(nextTab.path);
        });
      }
      // remove keepalive
      const tabItem = tabsMenuList.value.find(item => item.path === tabPath);
      if (tabItem?.isKeepAlive) {
        keepAliveStore.removeKeepAliveName(tabItem.path);
      }
      // set tabs
      tabsMenuList.value = tabsMenuList.value.filter(item => item.path !== tabPath);
    };

    // Close Tabs On Side
    const closeTabsOnSide = async (path: string, type: "left" | "right") => {
      const currentIndex = tabsMenuList.value.findIndex(item => item.path === path);
      if (currentIndex !== -1) {
        const range = type === "left" ? [0, currentIndex] : [currentIndex + 1, tabsMenuList.value.length];
        tabsMenuList.value = tabsMenuList.value.filter((item, index) => {
          return index < range[0] || index >= range[1] || !item.close;
        });
      }
      // set keepalive
      const KeepAliveList = tabsMenuList.value.filter(item => item.isKeepAlive);
      keepAliveStore.setKeepAliveName(KeepAliveList.map(item => item.path));
    };

    // Close MultipleTab
    const closeMultipleTab = async (tabsMenuValue?: string) => {
      tabsMenuList.value = tabsMenuList.value.filter(item => {
        return item.path === tabsMenuValue || !item.close;
      });
      // set keepalive
      const KeepAliveList = tabsMenuList.value.filter(item => item.isKeepAlive);
      keepAliveStore.setKeepAliveName(KeepAliveList.map(item => item.path));
    };

    // Set Tabs
    const setTabs = async (newTabsMenuList: TabsMenuProps[]) => {
      tabsMenuList.value = newTabsMenuList;
    };

    // Set Tabs Title
    const setTabsTitle = async (title: string) => {
      tabsMenuList.value.forEach(item => {
        if (item.path == getUrlWithParams()) item.title = title;
      });
    };

    return {
      tabsMenuList,
      addTabs,
      removeTabs,
      closeTabsOnSide,
      closeMultipleTab,
      setTabs,
      setTabsTitle
    };
  },
  {
    persist: {
      key: "geeker-tabs",
      storage: localStorage
    }
  }
);
