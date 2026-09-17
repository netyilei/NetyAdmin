/**
 * 内容分类树 → 下拉选项的统一扁平化工具。
 *
 * 原先 article-search 与 article-operate-modal 各持一份同构递归实现，
 * 现收敛为单一实现：extraKeys 控制是否携带 contentType/storageConfigId。
 */
export type CategoryOption<Extra extends boolean = false> = Extra extends true
  ? { label: string; value: number; contentType: string; storageConfigId: number | null }
  : { label: string; value: number };

interface CategoryNodeLike {
  id: number;
  name: string;
  contentType?: string;
  storageConfigId?: number | null;
  children?: CategoryNodeLike[] | null;
}

/** 全角空格缩进表达层级 */
export function buildCategoryOptions<Extra extends boolean = false>(
  categories: CategoryNodeLike[],
  withExtra: Extra = false as Extra
): CategoryOption<Extra>[] {
  const walk = (nodes: CategoryNodeLike[], level: number): CategoryOption<Extra>[] => {
    const result: CategoryOption<Extra>[] = [];
    for (const cat of nodes) {
      const prefix = '　'.repeat(level);
      const base: { label: string; value: number } = { label: prefix + cat.name, value: cat.id };
      if (withExtra) {
        result.push({
          ...base,
          contentType: cat.contentType ?? 'richtext',
          storageConfigId: cat.storageConfigId ?? null
        } as CategoryOption<Extra>);
      } else {
        result.push(base as CategoryOption<Extra>);
      }
      if (cat.children && cat.children.length > 0) {
        result.push(...walk(cat.children, level + 1));
      }
    }
    return result;
  };
  return walk(categories, 0);
}
