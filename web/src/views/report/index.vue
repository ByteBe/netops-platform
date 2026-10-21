<script setup lang="ts">
// 巡检报告：链路通断 + 服务器使用情况 + 数据库状态，生成/邮件发送/查看
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, delEnc } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
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
  include: ['link', 'monitor', 'db', 'nginx'] as string[]
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
  await postEnc(`/report/${r.id}/send`, {})
  ElMessage.success(t('common.success'))
  await load()
}

async function remove(r: ReportRecord) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/report/${r.id}`)
  ElMessage.success(t('common.success'))
  await load()
}

const auth = useAuthStore()

function downloadPdf(r: ReportRecord) {
  window.open(`/api/v1/report/${r.id}/pdf?token=${encodeURIComponent(auth.token)}`, '_blank')
}

async function view(r: ReportRecord) {
  viewVisible.value = true
  viewContent.value = '<p>' + t('report.loading') + '</p>'
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

let autoTimer: number | undefined
onMounted(() => { load(); autoTimer = window.setInterval(load, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })

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
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('report.title') }}</div>
        <div class="np-page-head-desc">{{ t('report.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button @click="quickGen('weekly')">{{ t('report.weekly') }}</el-button>
        <el-button @click="quickGen('monthly')">{{ t('report.monthly') }}</el-button>
        <el-button type="primary" @click="genDialog = true">
          <el-icon><Document /></el-icon>{{ t('report.generate') }}
        </el-button>
      </div>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Document /></el-icon>
        <span>{{ t('report.record') }}</span>
      </div>
      <el-table :data="records" v-loading="loading" stripe class="np-table">
        <el-table-column prop="title" :label="t('report.title')" min-width="220" />
        <el-table-column prop="created_at" :label="t('report.date')" min-width="180">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="sent_to" :label="t('report.target')" min-width="180">
          <template #default="{ row }">{{ row.sent_to || '—' }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="320">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="view(row)">{{ t('report.view') }}</el-button>
            <el-button size="small" text type="warning" @click="downloadPdf(row)">{{ t('report.downloadPdf') }}</el-button>
            <el-button size="small" text type="success" @click="send(row)">{{ t('report.sendMail') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!records.length && !loading" class="np-empty-box">
        <el-icon class="np-empty-icon"><Document /></el-icon>
        <div class="np-empty-title">{{ t('common.noData') }}</div>
        <div class="np-empty-desc">{{ t('report.desc') }}</div>
      </div>
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
            <el-checkbox value="nginx">{{ t('report.nginxStatus') }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="genDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="genLoading" @click="generate">{{ t('common.generate') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="viewVisible" :title="t('report.view')" width="950px" top="3vh" :destroy-on-close="true">
      <iframe ref="viewFrame" v-if="viewContent" :srcdoc="viewContent" style="width:100%;height:75vh;border:none;background:var(--np-bg-elevated)"></iframe>
      <div v-else style="padding:40px;text-align:center;color:var(--np-text-2)">{{ t('report.loading') }}</div>
      <template #footer>
        <el-button type="primary" @click="exportPDF">{{ t('report.exportPDF') }}</el-button>
        <el-button @click="viewVisible=false">{{ t('report.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

