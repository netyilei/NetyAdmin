import { h } from 'vue';
import { NTag } from 'naive-ui';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

/**
 * 字典 Hook
 */
export function useDict() {
  const dictStore = useDictStore();

  /**
   * 预加载字典数据(通常在页面初始化时调用)
   */
  async function loadDicts(codes: string[]) {
    await dictStore.loadDicts(codes);
  }

  /**
   * 获取字典标签
   */
  function getDictLabel(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    const label = item?.label || String(value);

    // 尝试翻译
    return label.includes('.') ? $t(label) : label;
  }

  /**
   * 渲染字典 Tag (适用于 NDataTable 渲染)
   */
  function renderDictTag(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));

    if (!item) return h('span', {}, value);

    const label = item.label.includes('.') ? $t(item.label) : item.label;

    return h(NTag, { type: item.tagType as any }, { default: () => label });
  }

  /**
   * 获取字典选项列表(用于单独手动构建查询组件)
   */
  function getDictOptions(dictCode: string) {
    return dictStore.dictMap.get(dictCode)?.map(i => ({ label: i.label, value: i.value })) || [];
  }

  return {
    loadDicts,
    getDictLabel,
    renderDictTag,
    getDictOptions
  };
}
