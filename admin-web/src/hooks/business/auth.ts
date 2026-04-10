import { useAuthStore } from '@/store/modules/auth';

export function useAuth() {
  const authStore = useAuthStore();

  function hasAuth(codes: string | string[]) {
    if (!authStore.isLogin) {
      return false;
    }

    const { roles, buttons } = authStore.userInfo;

    // super admin has all permissions
    if (roles.includes(import.meta.env.VITE_STATIC_SUPER_ROLE)) {
      return true;
    }

    if (typeof codes === 'string') {
      return buttons.includes(codes);
    }

    return codes.some(code => buttons.includes(code));
  }

  return {
    hasAuth
  };
}
