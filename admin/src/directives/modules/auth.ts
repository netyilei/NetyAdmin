/**
 * v-auth 按钮权限指令（V2 RBAC——value=权限码 admin.xxx，如
 * v-auth="'admin.admins.create'"；权限集=GET /admin/v1/permissions）。
 */
import type { Directive, DirectiveBinding } from "vue";

import { useAuthStore } from "@/stores/modules/auth";

const auth: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding) {
    const { value } = binding;
    const authStore = useAuthStore();
    const acts = authStore.authButtonListGet;
    const has = (v: string) => acts.includes(v);
    if (value instanceof Array && value.length) {
      if (!value.every(has)) el.remove();
    } else if (!has(value)) {
      el.remove();
    }
  }
};

export default auth;
