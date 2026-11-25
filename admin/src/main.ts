import { createApp } from "vue";

// iconfont css
import "@/assets/iconfont/iconfont.scss";
// element dark css（按需组件样式由 unplugin resolver 自动引入——dark vars 仍需全量基底；
// 命令式 API（ElMessage 等）样式需手动引样式文件）
import "element-plus/theme-chalk/dark/css-vars.css";
import "element-plus/es/components/message/style/css";
import "element-plus/es/components/message-box/style/css";
import "element-plus/es/components/notification/style/css";
// custom styles
import "@/styles/index.scss";
// element icons（全量注册——菜单 icon 字符串名动态解析需要；icons 包 gzip 约 30KB 可接受）
import * as Icons from "@element-plus/icons-vue";

// custom directives
import directives from "@/directives/index";
// vue i18n
import I18n from "@/languages/index";
// vue Router
import router from "@/routers";
// pinia store
import pinia from "@/stores";
// errorHandler
import errorHandler from "@/utils/errorHandler";

import App from "./App.vue";

const app = createApp(App);

app.config.errorHandler = errorHandler;

// register the element Icons component
Object.keys(Icons).forEach(key => {
  app.component(key, Icons[key as keyof typeof Icons]);
});

app.use(directives).use(pinia).use(router).use(I18n).mount("#app");
