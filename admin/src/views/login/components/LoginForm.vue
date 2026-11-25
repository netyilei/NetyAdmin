<template>
  <el-form ref="loginFormRef" :model="loginForm" :rules="loginRules" size="large">
    <el-form-item prop="username">
      <el-input v-model="loginForm.username" placeholder="用户名">
        <template #prefix>
          <el-icon class="el-input__icon">
            <user />
          </el-icon>
        </template>
      </el-input>
    </el-form-item>
    <el-form-item prop="password">
      <el-input v-model="loginForm.password" type="password" placeholder="密码" show-password autocomplete="new-password">
        <template #prefix>
          <el-icon class="el-input__icon">
            <lock />
          </el-icon>
        </template>
      </el-input>
    </el-form-item>
    <el-form-item prop="captchaCode">
      <div class="captcha-row">
        <el-input v-model="loginForm.captchaCode" placeholder="验证码" maxlength="4" @keyup.enter="login(loginFormRef)">
          <template #prefix>
            <el-icon class="el-input__icon">
              <key />
            </el-icon>
          </template>
        </el-input>
        <img v-if="captchaImg" class="captcha-img" :src="captchaImg" alt="验证码" title="点击刷新" @click="refreshCaptcha" />
        <el-button v-else class="captcha-img" @click="refreshCaptcha">获取验证码</el-button>
      </div>
    </el-form-item>
  </el-form>
  <div class="login-btn">
    <el-button :icon="CircleClose" round size="large" @click="resetForm(loginFormRef)"> 重置 </el-button>
    <el-button :icon="UserFilled" round size="large" type="primary" :loading="loading" @click="login(loginFormRef)">
      登录
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { CircleClose, UserFilled } from "@element-plus/icons-vue";
import type { ElForm } from "element-plus";
import { ElMessageBox, ElNotification } from "element-plus";
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";

import mittBus from "@/utils/mittBus";
import { Login } from "@/api/interface";
import { getCaptchaApi, loginApi } from "@/api/modules/login";
import { HOME_URL } from "@/config";
import { initDynamicRouter } from "@/routers/modules/dynamicRouter";
import { useKeepAliveStore } from "@/stores/modules/keepAlive";
import { useTabsStore } from "@/stores/modules/tabs";
import { useUserStore } from "@/stores/modules/user";

const router = useRouter();
const userStore = useUserStore();
const tabsStore = useTabsStore();
const keepAliveStore = useKeepAliveStore();

type FormInstance = InstanceType<typeof ElForm>;
const loginFormRef = ref<FormInstance>();
const loginRules = reactive({
  username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  password: [{ required: true, message: "请输入密码", trigger: "blur" }],
  captchaCode: [{ required: true, message: "请输入验证码", trigger: "blur" }]
});

const loading = ref(false);
const loginForm = reactive<Login.ReqLoginForm>({
  username: "",
  password: "",
  captchaId: "",
  captchaCode: ""
});

// —— 登录验证码（——英数 4 位一次性；图点击刷新；校验失败自动换新） ——
const captchaImg = ref("");
const refreshCaptcha = async () => {
  try {
    const resp: any = await getCaptchaApi();
    const d = resp.data ?? {};
    loginForm.captchaId = d.captchaId ?? "";
    loginForm.captchaCode = "";
    captchaImg.value = d.image ?? "";
  } catch {
    captchaImg.value = "";
  }
};
onMounted(refreshCaptcha);

// login
const login = (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  formEl.validate(async valid => {
    if (!valid) return;
    loading.value = true;
    try {
      // 1.执行登录接口（验证码一次性——校验失败自动换新图）
      const { data } = await loginApi({ ...loginForm }); // V2：密码明文走 TLS（服务端 bcrypt——前端不散列）;
      userStore.setToken(data.access_token);
      userStore.setRefreshToken(data.refresh_token); // 会话续期面（P1-4：随 persist 持久化）

      // 2.添加动态路由
      await initDynamicRouter();

      // 3.清空 tabs、keepAlive 数据
      tabsStore.setTabs([]);
      keepAliveStore.setKeepAliveName([]);

      // 3.5 回填用户信息（V2 profile——头像/姓名展示）
      userStore.setUserInfo({ username: data.profile.username, name: data.profile.name || data.profile.username });

      // 4.首改密提示（V2 must_change_password——重置/首启置位；改密=Header 密码
      // 弹窗（mitteBus 触发打开——A4 首改密流闭环）
      if (data.must_change_password) {
        sessionStorage.setItem("na2.must_change", "1");
        ElMessageBox.confirm("首次登录（或密码已被重置），请立即修改密码。", "安全提示", {
          confirmButtonText: "去修改",
          cancelButtonText: "稍后",
          type: "warning"
        })
          .then(() => mittBus.emit("openPasswordDialog"))
          .catch(() => mittBus.emit("openPasswordDialog"));
      }

      // 5.跳转到首页
      router.push(HOME_URL);
      ElNotification({
        title: "NetyAdmin",
        message: "欢迎登录 NetyAdmin 管理端",
        type: "success",
        duration: 3000
      });
    } catch {
      // 登录失败（密码/验证码错误等）——换新验证码（旧验证码已一次性消费）
      await refreshCaptcha();
    } finally {
      loading.value = false;
    }
  });
};

// resetForm
const resetForm = (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  formEl.resetFields();
};

onMounted(() => {
  // 监听 enter 事件（调用登录）
  document.onkeydown = (e: KeyboardEvent) => {
    if (e.code === "Enter" || e.code === "enter" || e.code === "NumpadEnter") {
      if (loading.value) return;
      login(loginFormRef.value);
    }
  };
});

onBeforeUnmount(() => {
  document.onkeydown = null;
});
</script>

<style scoped lang="scss">
@use "../index";
.captcha-row {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 10px;
  .el-input {
    flex: 1;
  }
  .captcha-img {
    width: 120px;
    height: 40px;
    flex-shrink: 0;
    cursor: pointer;
    border-radius: 4px;
    border: 1px solid var(--el-border-color);
  }
}
</style>
