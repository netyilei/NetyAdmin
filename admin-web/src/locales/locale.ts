import zhCN from './langs/zh-cn/index';
import enUS from './langs/en-us/index';

const locales: Record<string, App.I18n.Schema> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  zh: zhCN,
  en: enUS
};

export default locales;
