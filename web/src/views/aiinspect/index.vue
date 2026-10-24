<template>
  <div class="np-page aiinspect-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">AI 巡检</div>
        <div class="np-page-head-desc">一键采集系统快照，由 AI 分析设备、告警与日志并生成巡检报告</div>
      </div>
      <div class="np-page-head-actions">
        <el-button type="primary" :loading="loading" @click="runInspect">
          <el-icon><MagicStick /></el-icon> 开始 AI 巡检
        </el-button>
      </div>
    </div>

    <div class="main-row">
      <div class="main-col">
        <div v-if="report && report.snapshot" class="snapshot-box">
          <div class="np-kpi-grid">
            <div class="np-kpi">
              <div class="np-kpi-icon"><el-icon><Monitor /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_total ?? '-' }}</div>
                <div class="np-kpi-label">设备总数</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-success"><el-icon><CircleCheck /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_up ?? '-' }}</div>
                <div class="np-kpi-label">在线</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-danger"><el-icon><CircleClose /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_down ?? '-' }}</div>
                <div class="np-kpi-label">离线</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-warning"><el-icon><Bell /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.alert_active ?? '-' }}</div>
                <div class="np-kpi-label">活跃告警</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-info"><el-icon><BellFilled /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.alert_today ?? '-' }}</div>
                <div class="np-kpi-label">今日告警</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-danger"><el-icon><Warning /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.log_error_24h ?? '-' }}</div>
                <div class="np-kpi-label">24h 严重日志</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-warning"><el-icon><WarningFilled /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.log_warn_24h ?? '-' }}</div>
                <div class="np-kpi-label">24h 警告日志</div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="report" class="np-card report-box">
          <div class="report-header">
            <span class="time">巡检时间：{{ report.time }}</span>
          </div>
          <div class="md-body" v-html="rendered"></div>
        </div>

        <div v-if="!report && !loading" class="np-empty-box">
          <div class="np-empty-icon"><el-icon><MagicStick /></el-icon></div>
          <div class="np-empty-title">暂无巡检报告</div>
          <div class="np-empty-desc">点击上方按钮开始 AI 智能巡检</div>
        </div>
      </div>

      <div v-if="history.length" class="side-col">
        <div class="np-card side-box">
          <div class="np-card-title">历史报告</div>
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
import DOMPurify from 'dompurify'

const loading = ref(false)
const report = ref<any>(null)
const history = ref<any[]>([])
const rendered = computed(() => report.value ? DOMPurify.sanitize(marked.parse(report.value.report) as string) : '')

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
.main-row { display: flex; gap: 16px; align-items: flex-start; }
.main-col { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 16px; }
.side-col { width: 260px; flex-shrink: 0; }
.side-box { padding: 14px 16px; }
.history-list { max-height: 600px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.history-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid transparent;
  transition: background-color 0.15s ease-out, border-color 0.15s ease-out;
}
.history-item:hover { background: var(--np-hover); }
.history-item.active {
  background: var(--np-primary-weak);
  border-color: var(--np-border-strong);
}
.h-time { font-size: 13px; color: var(--np-text-1); font-variant-numeric: tabular-nums; }
.snapshot-box { margin-bottom: 0; }
.report-box { padding: 20px; }
.report-header { margin-bottom: 12px; color: var(--np-text-2); font-size: 14px; font-variant-numeric: tabular-nums; }
.md-body { font-size: 14px; line-height: 1.8; color: var(--np-text-1); }
.md-body :deep(h1) { font-size: 20px; margin: 16px 0 12px; color: var(--np-text-1); }
.md-body :deep(h2) { font-size: 17px; margin: 16px 0 10px; padding-bottom: 6px; border-bottom: 1px solid var(--np-border); color: var(--np-text-1); }
.md-body :deep(h3) { font-size: 15px; margin: 12px 0 8px; color: var(--np-text-1); }
.md-body :deep(table) { border-collapse: collapse; width: 100%; margin: 12px 0; }
.md-body :deep(th), .md-body :deep(td) { border: 1px solid var(--np-border); padding: 8px 12px; text-align: left; }
.md-body :deep(th) { background: var(--np-table-bg); font-weight: 600; color: var(--np-text-2); }
.md-body :deep(ul), .md-body :deep(ol) { padding-left: 24px; margin: 8px 0; }
.md-body :deep(li) { margin: 4px 0; }
.md-body :deep(p) { margin: 8px 0; }
.md-body :deep(strong) { color: var(--np-danger); }
.md-body :deep(blockquote) { margin: 8px 0; padding: 4px 12px; border-left: 2px solid var(--np-primary); background: var(--np-primary-weak); color: var(--np-text-2); }
.md-body :deep(code) { background: var(--np-primary-weak); border-radius: 4px; padding: 1px 5px; font-size: 13px; color: var(--np-text-1); }
@media (max-width: 900px) {
  .main-row { flex-direction: column; }
  .side-col { width: 100%; }
}
</style>
