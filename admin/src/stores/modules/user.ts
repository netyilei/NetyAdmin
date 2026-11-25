import { defineStore } from "pinia";
import { ref } from "vue";

import type { UserState } from "@/stores/interface";

export const useUserStore = defineStore(
  "geeker-user",
  () => {
    const token = ref<string>("");
    const setToken = (newToken: string) => {
      token.value = newToken;
    };

    // refresh_token 会话续期面（随 pinia persist 持久化——单一写
    // 路径，refresh 换新同走 store；此前从未存储=B4 修复实际不生效的根因）
    const refreshToken = ref<string>("");
    const setRefreshToken = (v: string) => {
      refreshToken.value = v;
    };

    const userInfo = ref<UserState["userInfo"]>({ name: "admin" });
    const setUserInfo = (newUserInfo: UserState["userInfo"]) => {
      userInfo.value = newUserInfo;
    };
    const mustChangePassword = ref<boolean>(false);
    const setMustChangePassword = (v: boolean) => {
      mustChangePassword.value = v;
    };

    return {
      token,
      refreshToken,
      userInfo,
      mustChangePassword,
      setToken,
      setRefreshToken,
      setUserInfo,
      setMustChangePassword
    };
  },
  {
    persist: {
      key: "geeker-user",
      storage: localStorage
    }
  }
);
