import * as Sentry from '@sentry/vue';
import type { App } from 'vue';

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
    environment: import.meta.env.MODE,
    release: import.meta.env.VITE_APP_VERSION || 'admin-web@dev',
    integrations: [
      Sentry.browserTracingIntegration(),
      Sentry.replayIntegration({
        maskAllText: true,
        blockAllMedia: true,
      }),
    ],
    // Performance tracing
    tracesSampleRate: import.meta.env.MODE === 'production' ? 0.2 : 1.0,
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
      'ResizeObserver loop limit exceeded',
    ],
  });
}

/**
 * Manually capture an error to Sentry (for use outside Vue components)
 */
export function captureError(error: Error | unknown, context?: Record<string, unknown>) {
  if (context) {
    Sentry.withScope((scope) => {
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
    role: user.role,
  });
}

/**
 * Clear the current user context (on logout)
 */
export function clearUserContext() {
  Sentry.setUser(null);
}
