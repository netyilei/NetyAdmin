import { h } from 'vue';
import { NTag } from 'naive-ui';
import { boolToDictValue, dictValueToBool, isDisabledStatus, isEnabledStatus } from '@/constants/business';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

/**
 * 通用状态标签渲染：按 映射表{value → {type,label}} 渲染 NTag（含空值兜底）。
 * 各页状态语义不同（数据留在页面），渲染结构统一在此——原先 task/upload-record/article
 * 等 9+ 处同构 NTag 三元/查表内联。
 */
export function renderTagFromMap(
  map: Record<string, { type: NaiveUI.ThemeColor; label: string }>,
  value: string | number | null | undefined
) {
  const hit = map[String(value ?? '')];
  if (!hit) return h('span', {}, String(value ?? '-'));
  return h(NTag, { type: hit.type }, { default: () => hit.label });
}

/**
 * 字典 label 的 i18n 启发式翻译：label 含 '.' 视为 i18n key 走 $t，否则原样。
 * 全项目唯一实现（原先 hook 内部与 app-dict-select / app-dict-radio-group 各自内联）。
 */
export function translateDictLabel(label: string): string {
  return label.includes('.') ? $t(label) : label;
}

export function useDict() {
  const dictStore = useDictStore();

  async function loadDicts(codes: string[]) {
    await dictStore.loadDicts(codes);
  }

  function getDictLabel(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    return translateDictLabel(item?.label || String(value));
  }

  function renderDictTag(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    if (!item) return h('span', {}, value);
    const label = translateDictLabel(item.label);
    return h(NTag, { type: item.tagType as any }, { default: () => label });
  }

  function renderBoolDictTag(dictCode: string, boolVal: boolean | null | undefined) {
    return renderDictTag(dictCode, boolToDictValue(boolVal));
  }

  function getDictBoolLabel(dictCode: string, boolVal: boolean | null | undefined) {
    return getDictLabel(dictCode, boolToDictValue(boolVal));
  }

  return {
    loadDicts,
    getDictLabel,
    renderDictTag,
    renderBoolDictTag,
    getDictBoolLabel,
    isEnabledStatus,
    isDisabledStatus,
    boolToDictValue,
    dictValueToBool
  };
}
