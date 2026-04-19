import type { Router } from 'vue-router';
import { useTitle } from '@vueuse/core';
import { APP_CONFIG } from '@/config';
import { $t } from '@/locales';

export function createDocumentTitleGuard(router: Router) {
  router.afterEach(to => {
    const { i18nKey, title } = to.meta;

    const documentTitle = i18nKey ? $t(i18nKey) : title;

    useTitle(`${documentTitle} - ${APP_CONFIG.name}`);
  });
}
