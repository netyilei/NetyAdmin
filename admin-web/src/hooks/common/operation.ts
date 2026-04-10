/** 很多操作需要区分 add 和 edit,里面有重复逻辑,包含loading状态,成功消息. 抽离这段逻辑 */
import type { Ref } from 'vue';
import { $t } from '@/locales';

interface OperationHandlers {
  editBeforeValidate?: () => boolean;
  edit: () => Promise<any>;
  add: () => Promise<any>;
  onSuccess?: () => void;
}

export async function useOperation(
  operateType: 'edit' | 'add' | 'addChild',
  loading: Ref<boolean>,
  handlers: OperationHandlers
) {
  let result;
  let successText;

  loading.value = true;
  try {
    if (operateType === 'edit') {
      if (handlers.editBeforeValidate && !handlers.editBeforeValidate()) {
        return false;
      }
      result = await handlers.edit();
      successText = $t('common.updateSuccess');
    } else {
      // 'add' 或 'addChild'
      result = await handlers.add();
      successText = $t('common.addSuccess');
    }

    if (!result.error) {
      loading.value = false;
      window.$message?.success(successText);
      handlers.onSuccess?.();
      return true;
    }
    loading.value = false;
    return false;
  } catch (e) {
    loading.value = false;
    throw e;
  }

  // finally {finally不能让loading=false在handlers.onSuccess?.()之前执行
  //   loading.value = false;
  // }
}
