<script setup lang="tsx">
import { onMounted, onUnmounted, reactive, ref } from 'vue';
import {
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NTag as NNaiveTag,
  NPagination,
  NPopconfirm,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  NTimeline,
  NTimelineItem
} from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import dayjs from 'dayjs';
import {
  fetchGetTaskList,
  fetchGetTaskLogs,
  fetchReloadTask,
  fetchRunTask,
  fetchStartTask,
  fetchStopTask,
  fetchUpdateTask
} from '@/service/api/v1/system-task';
import { useAppStore } from '@/store/modules/app';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

const appStore = useAppStore();
const loading = ref(false);
const taskList = ref<SystemManage.TaskInfo[]>([]);
const logModalVisible = ref(false);
const editModalVisible = ref(false);
const editForm = reactive({
  name: '',
  displayName: '',
  enabled: true,
  spec: ''
});
const currentTaskLogs = ref<SystemManage.TaskLog[]>([]);
const currentTaskName = ref('');
const logsLoading = ref(false);
const logPagination = ref({
  page: 1,
  size: 10,
  total: 0
});

const columns: DataTableColumns<SystemManage.TaskInfo> = [
  {
    key: 'displayName',
    title: $t('page.ops.task.name'),
    align: 'left',
    minWidth: 150,
    render: row => (
      <div class="flex-col">
        <span class="font-bold">{row.displayName || row.name}</span>
        <span class="text-12px text-gray-400">
          {row.type === 'cron' ? row.spec : `${$t('page.ops.task.every')} ${row.spec}`}
        </span>
      </div>
    )
  },
  {
    key: 'type',
    title: $t('page.ops.task.type'),
    align: 'center',
    width: 100,
    render: row => {
      const typeMap: Record<string, { type: NaiveUI.ThemeColor; label: string }> = {
        cron: { type: 'info', label: 'Cron' },
        interval: { type: 'primary', label: $t('page.ops.task.typeInterval') },
        once: { type: 'warning', label: $t('page.ops.task.typeOnce') }
      };
      const config = typeMap[row.type];
      return (
        <NTag type={config.type} size="small">
          {config.label}
        </NTag>
      );
    }
  },
  {
    key: 'enabled',
    title: $t('page.ops.task.availability'),
    align: 'center',
    width: 120,
    render: row => (
      <NSpace vertical align="center" size={4}>
        <NTag type={row.enabled ? 'success' : 'default'} size="small" bordered={false}>
          {row.enabled ? $t('page.ops.task.enabled') : $t('page.ops.task.disabled')}
        </NTag>
        {row.isRunning && (
          <NNaiveTag
            type="primary"
            size="tiny"
            class="animate-pulse"
            v-slots={{
              icon: () => <div class="i-mdi-loading animate-spin" />
            }}
          >
            {$t('page.ops.task.running')}
          </NNaiveTag>
        )}
      </NSpace>
    )
  },
  {
    key: 'lastRun',
    title: $t('page.ops.task.lastRun'),
    align: 'left',
    minWidth: 200,
    render: row => {
      // 特殊处理单次执行任务（如数据库迁移），通常在启动阶段完成
      if (row.type === 'once') {
        return (
          <div class="flex-y-center gap-4px">
            <NNaiveTag type="success" size="tiny" bordered={false}>
              {$t('page.ops.task.success')}
            </NNaiveTag>
            <span class="text-12px text-primary font-bold">{$t('page.ops.task.runAtStartup')}</span>
          </div>
        );
      }

      if (!row.lastRunTime || row.lastRunTime === '0001-01-01T00:00:00Z')
        return <span class="text-gray-300">{$t('page.ops.task.neverRun')}</span>;
      const statusType = row.lastStatus === 'success' ? 'success' : 'error';
      return (
        <div class="flex-col gap-2px">
          <div class="flex-y-center gap-4px">
            <NNaiveTag type={statusType} size="tiny" bordered={false}>
              {row.lastStatus === 'success' ? $t('page.ops.task.success') : $t('page.ops.task.failed')}
            </NNaiveTag>
            <span class="text-12px">{dayjs(row.lastRunTime).format('YYYY-MM-DD HH:mm:ss')}</span>
          </div>
          <div class="text-11px text-gray-400">
            {$t('page.ops.task.duration')}: {row.lastDuration?.toFixed(3)}s | {$t('page.ops.task.executionCount')}:{' '}
            {row.executionCount}
          </div>
          {row.lastStatus === 'error' && (
            <div class="w-180px truncate text-10px text-red-400" title={row.lastMessage}>
              {row.lastMessage}
            </div>
          )}
        </div>
      );
    }
  },
  {
    key: 'operate',
    title: $t('common.operate'),
    align: 'center',
    width: 370, // 再次增加操作栏宽度以避免按钮换行
    render: row => (
      <NSpace justify="center">
        {row.type !== 'once' && (
          <>
            {row.enabled ? (
              <>
                <NButton type="warning" size="small" onClick={() => handleStop(row.name)}>
                  {$t('page.ops.task.stop')}
                </NButton>
                <NPopconfirm
                  onPositiveClick={() => handleReload(row.name)}
                  v-slots={{
                    trigger: () => (
                      <NButton type="info" size="small" ghost>
                        {$t('page.ops.task.reload')}
                      </NButton>
                    ),
                    default: () => '确定要重启该任务吗？'
                  }}
                />
              </>
            ) : (
              <NButton type="success" size="small" onClick={() => handleStart(row.name)}>
                {$t('page.ops.task.start')}
              </NButton>
            )}
          </>
        )}
        <NButton type="primary" ghost size="small" loading={row.isRunning} onClick={() => handleRunNow(row.name)}>
          {$t('page.ops.task.runNow')}
        </NButton>
        <NButton size="small" onClick={() => handleEdit(row)}>
          {$t('common.edit')}
        </NButton>
        <NButton size="small" onClick={() => handleViewLogs(row.name)}>
          {$t('page.ops.task.viewLogs')}
        </NButton>
      </NSpace>
    )
  }
];

async function init() {
  loading.value = true;
  const { data } = await fetchGetTaskList();
  if (data) {
    taskList.value = data;
  }
  loading.value = false;
}

async function handleRunNow(name: string) {
  const { error } = await fetchRunTask(name);
  if (!error) {
    window.$message?.success('任务指令已下发');
    // 延迟刷新状态
    setTimeout(init, 500);
  }
}

async function handleStart(name: string) {
  const { error } = await fetchStartTask(name);
  if (!error) {
    window.$message?.success('任务已启动');
    init();
  }
}

async function handleStop(name: string) {
  const { error } = await fetchStopTask(name);
  if (!error) {
    window.$message?.success('任务已停止');
    init();
  }
}

async function handleReload(name: string) {
  const { error } = await fetchReloadTask(name);
  if (!error) {
    window.$message?.success('任务已重启');
    init();
  }
}

async function handleEdit(row: SystemManage.TaskInfo) {
  editForm.name = row.name;
  editForm.displayName = row.displayName || row.name;
  editForm.enabled = row.enabled;
  editForm.spec = row.spec;
  editModalVisible.value = true;
}

async function handleUpdateTask() {
  const { error } = await fetchUpdateTask({
    name: editForm.name,
    enabled: editForm.enabled,
    spec: editForm.spec
  });
  if (!error) {
    window.$message?.success('任务配置已更新');
    editModalVisible.value = false;
    init();
  }
}

async function handleViewLogs(name: string, page = 1) {
  currentTaskName.value = name;
  logModalVisible.value = true;
  logsLoading.value = true;
  logPagination.value.page = page;

  const { data } = await fetchGetTaskLogs({
    name,
    page,
    size: logPagination.value.size
  });

  if (data) {
    currentTaskLogs.value = data.list;
    logPagination.value.total = data.total;
  }
  logsLoading.value = false;
}

let timer: number | null = null;

onMounted(() => {
  init();
  // 定时刷新设为 30秒
  timer = window.setInterval(init, 30000);
});

onUnmounted(() => {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
});
</script>

<template>
  <div class="h-full flex-col-stretch gap-16px overflow-hidden">
    <NCard :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
      <template #header>
        <div class="w-full flex items-center justify-between">
          <span>{{ $t('page.ops.task.title') }}</span>
          <NButton type="primary" size="small" :loading="loading" @click="init">
            <template #icon>
              <icon-mdi-refresh />
            </template>
            {{ $t('common.refresh') }}
          </NButton>
        </div>
      </template>

      <NDataTable
        :columns="columns"
        :data="taskList"
        :loading="loading"
        :flex-height="!appStore.isMobile"
        class="sm:h-full"
        remote
        :row-key="row => row.name"
      />
    </NCard>

    <NModal
      v-model:show="logModalVisible"
      preset="card"
      :title="`${$t('page.ops.task.history')} - ${currentTaskName}`"
      style="width: 600px"
    >
      <div class="max-h-500px overflow-y-auto pr-4">
        <NSpin :show="logsLoading">
          <NTimeline v-if="currentTaskLogs.length > 0">
            <NTimelineItem
              v-for="log in currentTaskLogs"
              :key="log.id"
              :type="log.status === 'success' ? 'success' : 'error'"
              :content="log.message || '无详情'"
              :time="dayjs(log.startTime).format('YYYY-MM-DD HH:mm:ss')"
            >
              <template #header>
                <div class="w-full flex items-center justify-between pr-4">
                  <span class="font-bold">
                    {{ log.status === 'success' ? $t('page.ops.task.success') : $t('page.ops.task.failed') }}
                  </span>
                  <span class="text-12px text-gray-400 font-normal">
                    {{ $t('page.ops.task.duration') }}: {{ log.duration.toFixed(3) }}s
                  </span>
                </div>
              </template>
            </NTimelineItem>
          </NTimeline>
          <div v-else class="py-20 text-center text-gray-400">
            {{ $t('page.ops.task.noLogs') }}
          </div>
        </NSpin>
      </div>
      <template #footer>
        <div class="flex justify-end pr-4">
          <NPagination
            v-model:page="logPagination.page"
            :item-count="logPagination.total"
            :page-size="logPagination.size"
            @update:page="p => handleViewLogs(currentTaskName, p)"
          />
        </div>
      </template>
    </NModal>

    <NModal
      v-model:show="editModalVisible"
      preset="card"
      :title="`${$t('common.edit')} - ${editForm.displayName}`"
      style="width: 500px"
    >
      <NForm :model="editForm" label-placement="left" label-width="100">
        <NFormItem :label="$t('page.ops.task.name')">
          <NInput v-model:value="editForm.displayName" disabled />
        </NFormItem>
        <NFormItem :label="$t('page.ops.task.availability')">
          <NSwitch v-model:value="editForm.enabled" />
        </NFormItem>
        <NFormItem :label="$t('page.ops.task.spec')" path="spec">
          <NInput v-model:value="editForm.spec" placeholder="Cron表达式或时间间隔(如 30s)" />
          <template #feedback>
            <div class="mt-4px text-12px text-gray-400">
              Cron: "0 0/1 * * * ?" (每分钟)
              <br />
              Interval: "30s", "1m", "1h"
            </div>
          </template>
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="editModalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleUpdateTask">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.animate-pulse {
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}
</style>
