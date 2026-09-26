<script setup lang="ts">
// 巡检报告：链路通断 + 服务器使用情况 + 数据库状态，生成/邮件发送/查看
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, delEnc } from '@/utils/request'
import { fmtTime } from '@/utils'

const { t } = useI18n()

interface ReportRecord {
  id: number
  title: string
  content: string
  created_at: string
  sent_to: string
}

const records = ref<ReportRecord[]>([])
const loading = ref(false)

const genDialog = ref(false)
const genLoading = ref(false)
const genForm = reactive({
  title: '网络巡检报告',
  send_email: false,
  email_to: '',
  include: ['link', 'monitor', 'db'] as string[]
})

const viewVisible = ref(false)
const viewContent = ref('')

async function load() {
  loading.value = true
  try {
    records.value = await getEnc<ReportRecord[]>('/report/list')
  } finally {
    loading.value = false
  }
}

async function generate() {
  if (!genForm.include.length) {
    ElMessage.warning('请至少选择一类巡检内容')
    return
  }
  genLoading.value = true
  try {
    const r = await postEnc<{ id: number }>('/report/generate', genForm)
    ElMessage.success(t('common.success'))
    genDialog.value = false
    await load()
  } finally {
    genLoading.value = false
  }
}

async function send(r: ReportRecord) {
  const to = await ElMessageBox.prompt('请输入接收邮箱（留空使用默认）', t('report.sendMail'), {
    inputValue: '',
    confirmButtonText: t('common.confirm'),
    cancelButtonText: t('common.cancel')
  })
  await postEnc(`/report/${r.id}/send`, { email_to: to.value })
  ElMessage.success(t('common.success'))
}

async function remove(r: ReportRecord) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/report/${r.id}`)
  ElMessage.success(t('common.success'))
  await load()
}

async function view(r: ReportRecord) {
  viewVisible.value = true
  viewContent.value = '<p style="color:#999">加载中...</p>'
  try {
    const detail = await getEnc<ReportRecord>(`/report/${r.id}`)
    let html = detail.content || r.content || ''
    // 去掉完整HTML文档外壳，只保留body内部内容
    html = html.replace(/<!DOCTYPE[^>]*>/gi, '').replace(/<html[^>]*>/gi, '').replace(/<\/html>/gi, '')
    html = html.replace(/<head[^>]*>[\s\S]*?<\/head>/gi, '')
    html = html.replace(/<body[^>]*>/gi, '').replace(/<\/body>/gi, '')
    viewContent.value = html || '<p>报告内容为空</p>'
  } catch (e: any) {
    viewContent.value = '<p>加载失败: ' + (e.message || '未知错误') + '</p>'
  }
}

onMounted(load)
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <span class="np-page-desc">包含链路通断、服务器使用情况、数据库状态等巡检内容</span>
      <div class="spacer"></div>
      <el-button type="primary" @click="genDialog = true">
        <el-icon><Document /></el-icon>{{ t('report.generate') }}
      </el-button>
    </div>

    <div class="np-card">
      <el-table :data="records" v-loading="loading" stripe class="np-table">
        <el-table-column prop="title" :label="t('report.title')" min-width="220" />
        <el-table-column prop="created_at" :label="t('report.date')" min-width="180">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="sent_to" :label="t('report.target')" min-width="180">
          <template #default="{ row }">{{ row.sent_to || '—' }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="240" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="view(row)">{{ t('report.view') }}</el-button>
            <el-button size="small" text type="success" @click="send(row)">{{ t('report.sendMail') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!records.length" class="np-empty">{{ t('common.noData') }}</div>
    </div>

    <el-dialog v-model="genDialog" :title="t('report.generate')" width="520px">
      <el-form :model="genForm" label-width="110px">
        <el-form-item :label="t('report.title')">
          <el-input v-model="genForm.title" />
        </el-form-item>
        <el-form-item label="巡检内容">
          <el-checkbox-group v-model="genForm.include">
            <el-checkbox value="link">{{ t('report.linkStatus') }}</el-checkbox>
            <el-checkbox value="monitor">{{ t('report.serverUsage') }}</el-checkbox>
            <el-checkbox value="db">数据库状态</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item :label="t('report.sendMail')">
          <el-switch v-model="genForm.send_email" />
        </el-form-item>
        <el-form-item :label="t('report.target')" v-if="genForm.send_email">
          <el-input v-model="genForm.email_to" placeholder="多个邮箱用逗号分隔" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="genDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="genLoading" @click="generate">{{ t('common.generate') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="viewVisible" :title="t('report.view')" width="950px" top="3vh" :destroy-on-close="true">
      <iframe v-if="viewContent" :srcdoc="viewContent" style="width:100%;height:75vh;border:none;background:#fff"></iframe>
      <div v-else style="padding:40px;text-align:center;color:#999">加载中...</div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-report-content {
  min-height: 200px;
  max-height: 72vh;
  overflow-y: auto;
  line-height: 1.7;
  :deep(table) {
    border-collapse: collapse;
    width: 100%;
    margin: 8px 0;
    th, td {
      border: 1px solid var(--np-border);
      padding: 6px 10px;
      font-size: 13px;
    }
    th {
      background: var(--np-primary-weak);
    }
  }
  :deep(h1), :deep(h2), :deep(h3) {
    margin: 14px 0 8px;
  }
}
</style>
