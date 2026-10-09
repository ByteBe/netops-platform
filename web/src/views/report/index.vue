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
  title: t('report.reportTitle'),
  send_email: false,
  channels: [] as string[],
  email_to: '',
  include: ['link', 'monitor', 'db'] as string[]
})

const viewVisible = ref(false)
const viewFrame = ref<HTMLIFrameElement>()
const viewContent = ref('')
function exportPDF() {
  const w = viewFrame.value?.contentWindow
  if (w) w.print()
}

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
    ElMessage.warning(t('report.selectContent'))
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
  const to = await ElMessageBox.prompt(t('report.emailPrompt'), t('report.sendMail'), {
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
  viewContent.value = '<p style="color:#999">' + t('report.loading') + '</p>'
  try {
    const detail = await getEnc<ReportRecord>(`/report/${r.id}`)
    let html = detail.content || r.content || ''
    // 保留完整HTML（含样式），直接用iframe srcdoc渲染
    if (!html.toLowerCase().includes('<html')) {
      html = '<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>' + html + '</body></html>'
    }
    viewContent.value = html || '<p>' + t('report.emptyContent') + '</p>'
  } catch (e: any) {
    viewContent.value = '<p>' + t('report.loadFailed') + (e.message || t('report.unknownError')) + '</p>'
  }
}

onMounted(load)

async function quickGen(kind: string) {
  const now = new Date()
  let start: Date, title: string
  if (kind === 'weekly') {
    start = new Date(now.getTime() - 7 * 86400000)
    title = `${t('report.weeklyTitle')} ${start.toISOString().slice(0,10)}~${now.toISOString().slice(0,10)}`
  } else {
    start = new Date(now.getTime() - 30 * 86400000)
    title = `${t('report.monthlyTitle')} ${start.toISOString().slice(0,7)}`
  }
  const fmt = (d: Date) => d.toISOString().slice(0,10) + ' 00:00:00'
  await postEnc('/report/generate', { title, start: fmt(start), end: fmt(now), include: ['link','monitor','db','container','node'] })
  ElMessage.success(t('report.generated')); load()
}
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <span class="np-page-desc">{{ t('report.desc') }}</span>
      <div class="spacer"></div>
      <el-button @click="quickGen('weekly')">{{ t('report.weekly') }}</el-button>
      <el-button @click="quickGen('monthly')">{{ t('report.monthly') }}</el-button>
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
        <el-form-item :label="t('report.inspectItems')">
          <el-checkbox-group v-model="genForm.include">
            <el-checkbox value="link">{{ t('report.linkStatus') }}</el-checkbox>
            <el-checkbox value="monitor">{{ t('report.serverUsage') }}</el-checkbox>
            <el-checkbox value="db">{{ t('report.dbStatus') }}</el-checkbox>
            <el-checkbox value="container">{{ t('report.containerK8s') }}</el-checkbox>
            <el-checkbox value="node">{{ t('report.nodeHealth') }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item :label="t('report.channels')">
          <el-select v-model="genForm.channels" multiple :placeholder="t('report.channelsPlaceholder')" style="width:100%">
            <el-option :label="t('report.email')" value="email" />
            <el-option :label="t('report.dingtalk')" value="dingtalk" />
            <el-option :label="t('report.wecom')" value="wecom" />
            <el-option :label="t('report.feishu')" value="feishu" />
            <el-option :label="t('report.sms')" value="sms" />
            <el-option label="Webhook" value="webhook" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('report.target')" v-if="genForm.channels?.includes('email')">
          <el-input v-model="genForm.email_to" :placeholder="t('report.emailPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="genDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="genLoading" @click="generate">{{ t('common.generate') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="viewVisible" :title="t('report.view')" width="950px" top="3vh" :destroy-on-close="true">
      <iframe ref="viewFrame" v-if="viewContent" :srcdoc="viewContent" style="width:100%;height:75vh;border:none;background:#fff"></iframe>
      <div v-else style="padding:40px;text-align:center;color:#999">{{ t('report.loading') }}</div>
      <template #footer>
        <el-button type="primary" @click="exportPDF">{{ t('report.exportPDF') }}</el-button>
        <el-button @click="viewVisible=false">{{ t('report.close') }}</el-button>
      </template>
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
