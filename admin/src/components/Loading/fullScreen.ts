import { ElLoading } from "element-plus";
import { storeToRefs } from "pinia";

import { useGlobalStore } from "@/stores/modules/global";

/* 全局请求 loading */
let loadingInstance: ReturnType<typeof ElLoading.service>;

/**
 * @description 开启 Loading
 * */
const startLoading = () => {
  const globalStore = useGlobalStore();
  const { isDark } = storeToRefs(globalStore);

  loadingInstance = ElLoading.service({
    fullscreen: true,
    lock: true,
    text: "Loading",
    background: isDark.value ? "rgba(0, 0, 0, 0.4)" : "rgba(255, 255, 255, 0.4)"
  });
};

/**
 * @description 结束 Loading
 * */
const endLoading = () => {
  loadingInstance.close();
};

/**
 * @description 显示全屏加载
 * */
let needLoadingRequestCount = 0;
export const showFullScreenLoading = () => {
  if (needLoadingRequestCount === 0) {
    startLoading();
  }
  needLoadingRequestCount++;
};

/**
 * @description 隐藏全屏加载
 * */
export const tryHideFullScreenLoading = () => {
  if (needLoadingRequestCount <= 0) return;
  needLoadingRequestCount--;
  if (needLoadingRequestCount === 0) {
    endLoading();
  }
};
