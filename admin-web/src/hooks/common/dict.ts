import { h } from 'vue';
import { NTag } from 'naive-ui';
import { boolToDictValue, dictValueToBool, isDisabledStatus, isEnabledStatus } from '@/constants/business';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

export function useDict() {
  const dictStore = useDictStore();

  async function loadDicts(codes: string[]) {
    await dictStore.loadDicts(codes);
  }

  function getDictLabel(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    const label = item?.label || String(value);
    return label.includes('.') ? $t(label) : label;
  }

  function renderDictTag(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    if (!item) return h('span', {}, value);
    const label = item.label.includes('.') ? $t(item.label) : item.label;
    return h(NTag, { type: item.tagType as any }, { default: () => label });
  }

  function getDictOptions(dictCode: string) {
    return dictStore.dictMap.get(dictCode)?.map(i => ({ label: i.label, value: i.value })) || [];
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
    getDictOptions,
    renderBoolDictTag,
    getDictBoolLabel,
    isEnabledStatus,
    isDisabledStatus,
    boolToDictValue,
    dictValueToBool
  };
}
