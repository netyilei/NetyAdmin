<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import Editor from '@toast-ui/editor';
import '@toast-ui/editor/dist/toastui-editor.css';
import '@toast-ui/editor/dist/theme/toastui-editor-dark.css';
import { fetchCreateUploadRecord, fetchGetUploadCredentials } from '@/service/api/v1/storage';
import { useThemeStore } from '@/store/modules/theme';
import { uploadWithPresignedUrl } from '@/utils/upload';

interface Props {
  modelValue?: string;
  height?: string;
  placeholder?: string;
  disabled?: boolean;
  initialEditType?: 'markdown' | 'wysiwyg';
  storageConfigId?: number;
}

interface Emits {
  (e: 'update:modelValue', value: string): void;
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  height: '400px',
  placeholder: '',
  disabled: false,
  initialEditType: 'markdown',
  storageConfigId: undefined
});

const emit = defineEmits<Emits>();

const themeStore = useThemeStore();
const editorRef = ref<HTMLElement>();
const editorInstance = ref<Editor>();
const isDark = computed(() => themeStore.darkMode);

// Store last value to avoid infinite update loops
let lastValue = props.modelValue || '';

async function handleImageUpload(blob: Blob | File, callback: (url: string, text?: string) => void) {
  try {
    const file =
      blob instanceof File ? blob : new File([blob], `image-${Date.now()}.png`, { type: blob.type || 'image/png' });

    const { data: credentials, error } = await fetchGetUploadCredentials({
      configId: props.storageConfigId,
      fileName: file.name,
      fileSize: file.size,
      contentType: file.type,
      businessType: 'editor_image'
    });

    if (error || !credentials) {
      throw new Error('获取上传凭证失败');
    }

    const loadingMsg = window.$message?.loading('图片上传中...', { duration: 0 });

    const fileUrl = await uploadWithPresignedUrl(credentials, file, () => {});
    loadingMsg?.destroy();

    await fetchCreateUploadRecord({
      configId: credentials.configId,
      fileName: file.name,
      objectKey: credentials.objectKey,
      fileSize: file.size,
      mimeType: file.type,
      businessType: 'editor_image'
    });

    callback(fileUrl, file.name);
  } catch (err: any) {
    window.$message?.error(err.message || '图片上传失败');
    callback('');
  }
}

function initEditor() {
  if (!editorRef.value) return;

  // Ensure current props value is used as initial value
  lastValue = props.modelValue || '';

  editorInstance.value = new Editor({
    el: editorRef.value,
    height: props.height,
    initialEditType: props.initialEditType,
    previewStyle: 'vertical',
    initialValue: lastValue,
    usageStatistics: false,
    placeholder: props.placeholder,
    theme: isDark.value ? 'dark' : 'light',
    hideModeSwitch: false,
    hooks: {
      addImageBlobHook: handleImageUpload
    },
    events: {
      change: () => {
        const markdown = editorInstance.value?.getMarkdown() || '';
        if (markdown !== lastValue) {
          lastValue = markdown;
          emit('update:modelValue', markdown);
        }
      }
    }
  });

  if (props.disabled) {
    editorInstance.value.disable();
  }
}

function destroyEditor() {
  if (editorInstance.value) {
    try {
      editorInstance.value.destroy();
    } catch {
      // ignore
    }
    editorInstance.value = undefined;
    if (editorRef.value) {
      editorRef.value.innerHTML = '';
    }
  }
}

watch(
  () => props.modelValue,
  newValue => {
    if (!editorInstance.value) return;

    // Only update if external value is different and not matching what we last handled
    if (newValue !== lastValue) {
      lastValue = newValue || '';
      try {
        editorInstance.value.setMarkdown(lastValue, false);
      } catch {
        // ignore
      }
    }
  }
);

watch(
  () => props.disabled,
  disabled => {
    if (editorInstance.value) {
      try {
        if (disabled) {
          editorInstance.value.disable();
        } else {
          editorInstance.value.enable();
        }
      } catch {
        // ignore
      }
    }
  }
);

watch(isDark, () => {
  destroyEditor();
  nextTick(() => {
    initEditor();
  });
});

onMounted(() => {
  initEditor();
});

onUnmounted(() => {
  destroyEditor();
});
</script>

<template>
  <div ref="editorRef" class="toast-ui-editor-wrapper"></div>
</template>

<style scoped>
.toast-ui-editor-wrapper {
  width: 100%;
}

.toast-ui-editor-wrapper :deep(.toastui-editor-defaultUI) {
  border-radius: 4px;
}

.toast-ui-editor-wrapper :deep(.toastui-editor-toolbar) {
  border-radius: 4px 4px 0 0;
}

.toast-ui-editor-wrapper :deep(.toastui-editor-contents) {
  font-family: inherit;
}
</style>
