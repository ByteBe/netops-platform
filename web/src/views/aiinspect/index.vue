<template>
  <div class="aiinspect-page">
    <div class="np-toolbar">
      <h3>AI 巡检</h3>
      <el-button type="primary" :loading="loading" @click="runInspect">
        <el-icon><MagicStick /></el-icon> 开始 AI 巡检
      </el-button>
    </div>

    <div class="main-row">
      <div class="main-col">
        <div v-if="report" class="np-card snapshot-box">
          <h4>系统快照</h4>
          <el-descriptions :column="3" border size="small">
            <el-descriptions-item label="设备总数">{{ report.snapshot.device_total }}</el-descriptions-item>
            <el-descriptions-item label="在线">{{ report.snapshot.device_up }}</el-descriptions-item>
            <el-descriptions-item label="离线">{{ report.snapshot.device_down }}</el-descriptions-item>
            <el-descriptions-item label="活跃告警">{{ report.snapshot.alert_active }}</el-descriptions-item>
            <el-descriptions-item label="今日告警">{{ report.snapshot.alert_today }}</el-descriptions-item>
            <el-descriptions-item label="24h严重日志">{{ report.snapshot.log_error_24h }}</el-descriptions-item>
            <el-descriptions-item label="24h警告日志">{{ report.snapshot.log_warn_24h }}</el-descriptions-item>
          </el-descriptions>
        </div>

        <div v-if="report" class="np-card report-box">
          <div class="report-header">
            <span class="time">巡检时间：{{ report.time }}</span>
          </div>
          <div class="md-body" v-html="rendered"></div>
        </div>

        <el-empty v-if="!report && !loading" description="点击上方按钮开始 AI 智能巡检" />
      </div>

      <div v-if="history.length" class="side-col">
        <div class="np-card side-box">
          <h4>历史报告</h4>
          <div class="history-list">
            <div v-for="h in history" :key="h.id" class="history-item" :class="{active: report && report.id === h.id}" @click="viewHistory(h)">
              <div class="h-time">{{ fmtTime(h.created_at) }}</div>
              <div class="h-actions" @click.stop>
                <el-button size="small" text type="danger" @click="delHistory(h)">删除</el-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { MagicStick } from '@element-plus/icons-vue'
import { getEnc, postEnc, delEnc } from '@/utils/request'
import { marked } from 'marked'

const loading = ref(false)
const report = ref<any>(null)
const history = ref<any[]>([])
const rendered = computed(() => report.value ? marked.parse(report.value.report) : '')

function fmtTime(s: string) { return s ? s.replace('T', ' ').substring(0, 19) : '' }

async function loadHistory() {
  history.value = await getEnc('/aiinspect/list')
}

async function runInspect() {
  loading.value = true
  try {
    report.value = await postEnc('/aiinspect/run', {})
    await loadHistory()
  } catch (e: any) {
    report.value = null
  } finally {
    loading.value = false
  }
}

async function viewHistory(row: any) {
  let snap: any = {}
  try { snap = JSON.parse(row.snapshot) } catch {}
  report.value = { id: row.id, report: row.report, snapshot: snap, time: fmtTime(row.created_at) }
}

async function delHistory(row: any) {
  await delEnc(`/aiinspect/${row.id}`)
  if (report.value && report.value.id === row.id) report.value = null
  await loadHistory()
}

let autoTimer: number | undefined
onMounted(() => { loadHistory(); autoTimer = window.setInterval(loadHistory, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
</script>

<style scoped>
.aiinspect-page { padding: 16px; }
.np-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.main-row { display: flex; gap: 16px; align-items: flex-start; }
.main-col { flex: 1; min-width: 0; }
.side-col { width: 260px; flex-shrink: 0; }
.side-box { padding: 12px; }
.side-box h4 { margin: 0 0 10px; font-size: 14px; }
.history-list { max-height: 600px; overflow-y: auto; }
.history-item { display: flex; justify-content: space-between; align-items: center; padding: 8px; border-radius: 4px; cursor: pointer; }
.history-item:hover { background: #f5f7fa; }
.history-item.active { background: #ecf5ff; }
.h-time { font-size: 13px; color: #333; }
.snapshot-box { padding: 16px; margin-bottom: 16px; }
.snapshot-box h4 { margin: 0 0 12px; }
.report-box { padding: 20px; }
.report-header { margin-bottom: 12px; color: #666; font-size: 14px; }
.md-body { font-size: 14px; line-height: 1.8; color: #333; }
.md-body :deep(h1) { font-size: 20px; margin: 16px 0 12px; }
.md-body :deep(h2) { font-size: 17px; margin: 16px 0 10px; padding-bottom: 6px; border-bottom: 1px solid #eee; }
.md-body :deep(h3) { font-size: 15px; margin: 12px 0 8px; }
.md-body :deep(table) { border-collapse: collapse; width: 100%; margin: 12px 0; }
.md-body :deep(th), .md-body :deep(td) { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
.md-body :deep(th) { background: #f5f7fa; }
.md-body :deep(ul), .md-body :deep(ol) { padding-left: 24px; margin: 8px 0; }
.md-body :deep(li) { margin: 4px 0; }
.md-body :deep(p) { margin: 8px 0; }
.md-body :deep(strong) { color: #c0392b; }
</style>
