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
              <div class="np-kpi-icon is-blue"><el-icon><Monitor /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_total ?? '-' }}</div>
                <div class="np-kpi-label">设备总数</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-green"><el-icon><CircleCheck /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_up ?? '-' }}</div>
                <div class="np-kpi-label">在线</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-red"><el-icon><CircleClose /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_down ?? '-' }}</div>
                <div class="np-kpi-label">离线</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-gray"><el-icon><InfoFilled /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.device_unknown ?? '-' }}</div>
                <div class="np-kpi-label">未知</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-orange"><el-icon><Bell /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.alert_active ?? '-' }}</div>
                <div class="np-kpi-label">活跃告警</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-amber"><el-icon><BellFilled /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.alert_today ?? '-' }}</div>
                <div class="np-kpi-label">今日告警</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-red"><el-icon><Warning /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.log_error_24h ?? '-' }}</div>
                <div class="np-kpi-label">24h 严重日志</div>
              </div>
            </div>
            <div class="np-kpi">
              <div class="np-kpi-icon is-yellow"><el-icon><WarningFilled /></el-icon></div>
              <div class="np-kpi-body">
                <div class="np-kpi-value">{{ report.snapshot.log_warn_24h ?? '-' }}</div>
                <div class="np-kpi-label">24h 警告日志</div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="report" class="np-card report-box">
          <div class="report-header">
            <span class="time"><el-icon style="vertical-align:-2px"><Clock /></el-icon> 巡检时间：{{ report.time }}</span>
            <span v-if="report.source" class="src-badge" :class="report.source === 'llm' ? 'is-llm' : 'is-fallback'">
              {{ report.source === 'llm' ? 'AI 智能分析' : '统计模式' }}
            </span>
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
import { MagicStick, Clock } from '@element-plus/icons-vue'
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
.report-header {
  margin-bottom: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px dashed var(--np-border);
  color: var(--np-text-2);
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}
.src-badge {
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 999px;
  font-weight: 600;
}
.src-badge.is-llm { background: linear-gradient(135deg, #e8f1ff, #d7e8ff); color: #1a5fb4; border: 1px solid #bcd7ff; }
.src-badge.is-fallback { background: linear-gradient(135deg, #fdf3e2, #fbe9cd); color: #b26a00; border: 1px solid #f5d9a5; }

/* ---- KPI 彩色立体图标 ---- */
.np-kpi-icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: #fff;
  position: relative;
  overflow: hidden;
  box-shadow: 0 4px 10px rgba(15, 76, 129, 0.22), inset 0 -2px 4px rgba(0, 0, 0, 0.14), inset 0 2px 3px rgba(255, 255, 255, 0.35);
}
.np-kpi-icon::after {
  content: '';
  position: absolute;
  left: 6px;
  top: 5px;
  width: 18px;
  height: 12px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.28);
  transform: rotate(-20deg);
}
.np-kpi-icon.is-blue { background: linear-gradient(135deg, #5aa7ff, #1e88e5); }
.np-kpi-icon.is-green { background: linear-gradient(135deg, #66bb6a, #2e7d32); }
.np-kpi-icon.is-red { background: linear-gradient(135deg, #ef5350, #c62828); }
.np-kpi-icon.is-gray { background: linear-gradient(135deg, #90a4ae, #546e7a); }
.np-kpi-icon.is-orange { background: linear-gradient(135deg, #ffb74d, #f57c00); }
.np-kpi-icon.is-amber { background: linear-gradient(135deg, #ffca28, #f9a825); }
.np-kpi-icon.is-yellow { background: linear-gradient(135deg, #fdd835, #f9a825); }
.np-kpi-icon.is-success { background: linear-gradient(135deg, #66bb6a, #2e7d32); }
.np-kpi-icon.is-danger { background: linear-gradient(135deg, #ef5350, #c62828); }
.np-kpi-icon.is-info { background: linear-gradient(135deg, #5aa7ff, #1e88e5); }
.np-kpi-icon.is-warning { background: linear-gradient(135deg, #ffb74d, #f57c00); }

/* ---- 报告正文 Markdown 美化 ---- */
.md-body { font-size: 14px; line-height: 1.8; color: var(--np-text-1); }
.md-body :deep(h1) {
  font-size: 20px;
  margin: 18px 0 14px;
  color: #0d3b66;
  padding: 8px 14px;
  border-left: 4px solid #1a73e8;
  background: linear-gradient(90deg, #eef4ff, #f8fbff);
  border-radius: 0 6px 6px 0;
  letter-spacing: 1px;
}
.md-body :deep(h2) {
  font-size: 17px;
  margin: 18px 0 10px;
  color: #0d3b66;
  padding: 7px 12px;
  border-left: 4px solid #4c8dff;
  background: linear-gradient(90deg, #f0f6ff, #fafcff);
  border-radius: 0 6px 6px 0;
}
.md-body :deep(h3) { font-size: 15px; margin: 14px 0 8px; color: #0d3b66; }
.md-body :deep(h1):first-child { margin-top: 4px; }
.md-body :deep(table) {
  border-collapse: separate;
  border-spacing: 0;
  width: 100%;
  margin: 12px 0;
  border: 1px solid #e3edfc;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 5px rgba(15, 76, 129, 0.08);
  font-size: 13px;
}
.md-body :deep(th) {
  background: linear-gradient(180deg, #1a73e8, #0d3b66);
  color: #fff;
  font-weight: 600;
  padding: 9px 12px;
  text-align: left;
  border: none;
}
.md-body :deep(td) { border: 1px solid #e3edfc; padding: 8px 12px; text-align: left; }
.md-body :deep(tbody tr:nth-child(even)) { background: #f8fbff; }
.md-body :deep(tbody tr:hover) { background: #eef4ff; }
.md-body :deep(ul), .md-body :deep(ol) { padding-left: 24px; margin: 8px 0; }
.md-body :deep(li) { margin: 4px 0; }
.md-body :deep(p) { margin: 8px 0; }
.md-body :deep(strong) { color: #ea4335; font-weight: 600; }
.md-body :deep(blockquote) {
  margin: 10px 0;
  padding: 8px 14px;
  border-left: 3px solid #1a73e8;
  background: linear-gradient(90deg, #f0f6ff, #fafcff);
  border-radius: 0 6px 6px 0;
  color: var(--np-text-2);
}
.md-body :deep(code) {
  background: #eef4ff;
  border-radius: 4px;
  padding: 1px 5px;
  font-size: 13px;
  color: #1a5fb4;
}
@media (max-width: 900px) {
  .main-row { flex-direction: column; }
  .side-col { width: 100%; }
}
</style>
