import type { App } from 'vue';
import * as Sentry from '@sentry/vue';

/**
 * Initialize Sentry error tracking for the Vue 3 admin frontend.
 * Only activates when VITE_SENTRY_DSN is configured.
 */
export function setupSentry(app: App) {
  const dsn = import.meta.env.VITE_SENTRY_DSN;

  if (!dsn) {
    // Sentry disabled - no DSN configured
    return;
  }

  Sentry.init({
    app,
    dsn,
    // environment 用标准 production 命名（而非 Vite 的 MODE='prod'），
    // 与 Sentry 看板和团队约定保持一致。
    environment: import.meta.env.PROD ? 'production' : import.meta.env.MODE,
    release: import.meta.env.VITE_APP_VERSION || 'admin-web@dev',
    integrations: [
      Sentry.browserTracingIntegration(),
      Sentry.replayIntegration({
        maskAllText: true,
        blockAllMedia: true
      })
    ],
    // Performance tracing
    // 修复 H1：旧代码用 `import.meta.env.MODE === 'production'`，
    // 但 package.json 的 build 命令是 `vite build --mode prod`（MODE='prod'），
    // 导致生产构建采样率退回 1.0（100%），与设计意图（生产 20%）相反。
    // 改用 Vite 内建的 PROD（任何非开发 mode 均为 true），与项目其它处（theme/shared/app.ts）一致。
    tracesSampleRate: import.meta.env.PROD ? 0.2 : 1.0,
    // Session replay
    replaysSessionSampleRate: 0.1,
    replaysOnErrorSampleRate: 1.0,
    // Ignore common non-actionable errors
    ignoreErrors: [
      // Network errors
      'Network Error',
      'timeout of',
      'Request aborted',
      // Navigation cancelled
      'Navigation cancelled',
      'navigation aborted',
      // Resize observer
      'ResizeObserver loop limit exceeded'
    ]
  });
}

/**
 * Manually capture an error to Sentry (for use outside Vue components)
 */
export function captureError(error: Error | unknown, context?: Record<string, unknown>) {
  if (context) {
    Sentry.withScope(scope => {
      Object.entries(context).forEach(([key, value]) => {
        scope.setExtra(key, value);
      });
      Sentry.captureException(error);
    });
  } else {
    Sentry.captureException(error);
  }
}

/**
 * Set the current user context for Sentry
 */
export function setUserContext(user: { id: string; username: string; role?: string }) {
  Sentry.setUser({
    id: user.id,
    username: user.username,
    role: user.role
  });
}

/**
 * Clear the current user context (on logout)
 */
export function clearUserContext() {
  Sentry.setUser(null);
}
