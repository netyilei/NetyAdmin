<template>
  <el-dialog v-model="dialogVisible" title="修改密码" width="420" draggable destroy-on-close>
    <el-form label-width="90">
      <el-form-item label="旧密码" required>
        <el-input v-model="form.oldPassword" type="password" show-password-on="click" />
      </el-form-item>
      <el-form-item label="新密码" required>
        <el-input v-model="form.newPassword" type="password" show-password-on="click" placeholder="≥8 位" />
      </el-form-item>
      <el-form-item label="确认密码" required>
        <el-input v-model="form.confirm" type="password" show-password-on="click" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="onSubmit">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { reactive, ref } from "vue";

import { adminApi } from "@/api/modules/identity";
import { elError } from "@/api/messageKeys";
import { LOGIN_URL } from "@/config";
import router from "@/routers";
import { useUserStore } from "@/stores/modules/user";

const userStore = useUserStore();
const dialogVisible = ref(false);
const saving = ref(false);
const form = reactive({ oldPassword: "", newPassword: "", confirm: "" });

const openDialog = () => {
  form.oldPassword = "";
  form.newPassword = "";
  form.confirm = "";
  dialogVisible.value = true;
};

const onSubmit = async () => {
  if (!form.oldPassword || form.newPassword.length < 8) {
    ElMessage.error("请填写旧密码与新密码（新密码 ≥8 位）");
    return;
  }
  if (form.newPassword !== form.confirm) {
    ElMessage.error("两次输入的新密码不一致");
    return;
  }
  saving.value = true;
  try {
    await adminApi.changePassword({ old_password: form.oldPassword, new_password: form.newPassword });
    // 改密=TokenVersion++ 全作废——回登录页重登（router+LOGIN_URL 常量——
    // 此前 window.location.href="/login" 在 hash 模式下路径直跳 404）
    ElMessage.success("密码已修改，请重新登录");
    userStore.setToken("");
    userStore.setRefreshToken("");
    router.replace(LOGIN_URL);
  } catch (e: any) {
    elError(e);
  } finally {
    saving.value = false;
  }
};

defineExpose({ openDialog });
</script>
