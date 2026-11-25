<template>
  <el-dialog v-model="dialogVisible" title="个人资料" width="520px" draggable destroy-on-close>
    <div class="avatar-row">
      <el-avatar :size="72" :src="form.avatarUrl || undefined" icon="UserFilled" />
      <div class="avatar-upload">
        <input ref="fileRef" type="file" accept="image/png,image/jpeg,image/webp" style="display: none" @change="onFile" />
        <el-button size="small" @click="fileRef?.click()">上传头像</el-button>
        <span class="avatar-tip">PNG/JPG/WebP ≤ 200KB</span>
      </div>
    </div>
    <el-form label-width="70">
      <el-form-item label="用户名">
        <el-input :model-value="userStore.userInfo.username" disabled />
      </el-form-item>
      <el-form-item label="姓名" required>
        <el-input v-model="form.name" />
      </el-form-item>
      <el-form-item label="邮箱">
        <el-input v-model="form.email" />
      </el-form-item>
      <el-form-item label="手机号">
        <el-input v-model="form.phone" placeholder="11 位手机号" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { elError } from "@/api/messageKeys";
import { reactive, ref } from "vue";

import { adminApi } from "@/api/modules/identity";
import { useUserStore } from "@/stores/modules/user";

const userStore = useUserStore();
const dialogVisible = ref(false);
const saving = ref(false);
const fileRef = ref<HTMLInputElement>();
const form = reactive({ name: "", email: "", phone: "", avatarUrl: "" });

const openDialog = async () => {
  // 回填（user-info 实时拉取——含 phone/avatarUrl）
  try {
    const resp: any = await adminApi.userInfo();
    const d = resp.data;
    Object.assign(form, {
      name: d.name ?? "",
      email: d.email ?? "",
      phone: d.phone ?? "",
      avatarUrl: d.avatarUrl ?? ""
    });
    userStore.setUserInfo({ username: d.username, name: d.name, avatar: d.avatarUrl });
  } catch {
    Object.assign(form, { name: userStore.userInfo.name, email: "", phone: "", avatarUrl: userStore.userInfo.avatar ?? "" });
  }
  dialogVisible.value = true;
};

// 头像选择→data URL（≤200KB 前端拦截）
const onFile = (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  if (file.size > 200 * 1024) {
    ElMessage.error("头像文件超过 200KB");
    return;
  }
  const reader = new FileReader();
  reader.onload = () => (form.avatarUrl = String(reader.result));
  reader.readAsDataURL(file);
};

const onSave = async () => {
  saving.value = true;
  try {
    const resp: any = await adminApi.updateProfile({
      name: form.name,
      email: form.email,
      phone: form.phone,
      avatarUrl: form.avatarUrl
    });
    userStore.setUserInfo({ username: resp.data.username, name: resp.data.name, avatar: resp.data.avatarUrl });
    ElMessage.success("个人资料已更新");
    dialogVisible.value = false;
  } catch (e: any) {
    elError(e);
  } finally {
    saving.value = false;
  }
};

defineExpose({ openDialog });
</script>

<style scoped lang="scss">
.avatar-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  .avatar-tip {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
}
</style>
