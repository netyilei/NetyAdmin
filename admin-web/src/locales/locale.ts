import zhCN from './langs/zh-cn/index';
import enUS from './langs/en-us/index';

const locales: Record<App.I18n.LangType, App.I18n.Schema> = {
  'zh-CN': zhCN,
  'en-US': enUS
};

export default locales;
