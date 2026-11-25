<template>
  <el-dropdown trigger="click">
    <div class="avatar">
      <img v-if="userStore.userInfo.avatar" :src="userStore.userInfo.avatar" alt="avatar" style="object-fit: cover" />
      <!-- 无自定义头像：显示用户名首字母 -->
      <div v-else class="avatar-letter">{{ avatarLetter }}</div>
    </div>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item @click="openDialog('infoRef')">
          <el-icon><User /></el-icon>{{ $t("header.personalData") }}
        </el-dropdown-item>
        <el-dropdown-item @click="openDialog('passwordRef')">
          <el-icon><Edit /></el-icon>{{ $t("header.changePassword") }}
        </el-dropdown-item>
        <el-dropdown-item divided @click="logout">
          <el-icon><SwitchButton /></el-icon>{{ $t("header.logout") }}
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
  <!-- infoDialog -->
  <InfoDialog ref="infoRef"></InfoDialog>
  <!-- passwordDialog -->
  <PasswordDialog ref="passwordRef"></PasswordDialog>
</template>

<script setup lang="ts">
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, ref } from "vue";
import { useRouter } from "vue-router";

import { logoutApi } from "@/api/modules/login";
import { LOGIN_URL } from "@/config";
import { useUserStore } from "@/stores/modules/user";

import InfoDialog from "./InfoDialog.vue";
import PasswordDialog from "./PasswordDialog.vue";

const router = useRouter();
const userStore = useUserStore();
// 默认头像=用户名/姓名首字母（无自定义头像时；取 name 优先、退 username）
const avatarLetter = computed(() =>
  (userStore.userInfo.name || userStore.userInfo.username || "A").trim().charAt(0).toUpperCase()
);
// 首改密流：登录页 mittBus 触发打开改密弹窗（A4 闭环）
import mittBus from "@/utils/mittBus";
mittBus.on("openPasswordDialog", () => openDialog("passwordRef"));

// 退出登录
const logout = () => {
  ElMessageBox.confirm("您是否确认退出登录?", "温馨提示", {
    confirmButtonText: "确定",
    cancelButtonText: "取消",
    type: "warning"
  }).then(async () => {
    // 1.执行退出登录接口
    await logoutApi();

    // 2.清除 Token（access+refresh 双清——只清 access 会让登录页请求 401 后
    // 拿残留 refresh 白发一次 refresh 再弹失效）
    userStore.setToken("");
    userStore.setRefreshToken("");

    // 3.重定向到登陆页
    router.replace(LOGIN_URL);
    ElMessage.success("退出登录成功！");
  });
};

// 打开修改密码和个人信息弹窗
const infoRef = ref<InstanceType<typeof InfoDialog> | null>(null);
const passwordRef = ref<InstanceType<typeof PasswordDialog> | null>(null);
const openDialog = (ref: string) => {
  if (ref == "infoRef") infoRef.value?.openDialog();
  if (ref == "passwordRef") passwordRef.value?.openDialog();
};
</script>

<style scoped lang="scss">
.avatar {
  width: 40px;
  height: 40px;
  overflow: hidden;
  cursor: pointer;
  border-radius: 50%;
  img {
    width: 100%;
    height: 100%;
  }
}
.avatar-letter {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 600;
  color: #fff;
  background: linear-gradient(135deg, #1d6fdc, #0e4cab);
  user-select: none;
}
</style>
