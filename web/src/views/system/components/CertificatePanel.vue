<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc } from '@/utils/request'
const { t } = useI18n()

const self = ref<any>(null)
const peers = ref<any[]>([])
const rotating = ref(false)
const generating = ref(false)
let timer: number | undefined
const parentDlg = ref(false)
const parentPem = ref('')
const parentUploading = ref(false)

async function load() {
  try {
    const r: any = await getEnc('/distributed/cert/self')
    self.value = r
    peers.value = await getEnc('/distributed/cert/peers')
  } catch (e: any) {
    ElMessage.error(t('node.certLoadFailed') + ': ' + (e?.message || e))
  }
}

function onParentFile(file: any) {
  const raw = file?.raw
  if (!raw) return
  const reader = new FileReader()
  reader.onload = () => { parentPem.value = String(reader.result || '') }
  reader.readAsText(raw)
}
async function uploadParentCert() {
  if (!parentPem.value.trim()) { ElMessage.warning(t('node.certImportPlaceholder')); return }
  parentUploading.value = true
  try {
    const r: any = await postEnc('/distributed/cert/parent-cert', { cert_pem: parentPem.value.trim() })
    if (r.ok) {
      ElMessage.success(t('node.certParentImported'))
      parentDlg.value = false
      parentPem.value = ''
      await load()
    } else {
      ElMessage.error(t('node.certParentFail') + ': ' + (r.error || ''))
    }
  } catch (e: any) {
    ElMessage.error(t('node.certParentFail') + ': ' + (e?.message || e))
  } finally { parentUploading.value = false }
}

// 生成本机私有证书：本机证书可独立生成，不依赖是否已导入上级公钥（后端已移除前置强制）
async function genPrivate() {
  generating.value = true
  try {
    const r: any = await postEnc('/distributed/cert/gen-private', {})
    if (r.ok) {
      ElMessage.success(t('node.certPrivateReady') + ' (' + short(r.fingerprint) + ')')
      await load()
    } else {
      ElMessage.error(r.error || t('node.certGenFailed'))
    }
  } catch (e: any) {
    ElMessage.error(t('node.certGenFailed') + ': ' + (e?.message || e))
  } finally {
    generating.value = false
  }
}

async function rotate() {
  try {
    await ElMessageBox.confirm(t('node.certRotateConfirm'), t('node.certRotate'), { type: 'warning' })
  } catch {
    return
  }
  rotating.value = true
  try {
    const r: any = await postEnc('/distributed/cert/rotate', {})
    ElMessage.success(t('node.certRotated') + (r.fingerprint ? ' (' + short(r.fingerprint) + ')' : ''))
    await load()
  } catch (e: any) {
    ElMessage.error(t('node.certRotateFailed') + ': ' + (e?.message || e))
  } finally {
    rotating.value = false
  }
}

function short(fp: string) {
  return fp ? fp.slice(0, 8) + '…' + fp.slice(-8) : ''
}
function downloadPem() {
  if (!self.value?.cert_pem) {
    ElMessage.warning(t('node.certNotReady'))
    return
  }
  const blob = new Blob([self.value.cert_pem + '\n'], { type: 'application/x-pem-file' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  const safeName = (self.value.name || 'node').replace(/[\\/:*?"<>|]/g, '_')
  a.download = `cert-${safeName}.pem`
  a.click()
  URL.revokeObjectURL(a.href)
}
function fmt(tm: string) {
  return tm ? tm.replace('T', ' ').substring(0, 19) : '-'
}
function statusText(s: string) {
  return s === 'valid' ? t('node.certValid') : s === 'expired' ? t('node.certExpired') : t('node.certInvalid')
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 60000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>
<template>
  <div class="np-card">
    <div class="np-card-title">{{ t('node.certTitle') }}</div>

    <div v-if="self" class="cert-self">
      <!-- 公有证书（公钥）：可下载分发，不可查看明文 -->
      <div class="section-title">{{ t('node.certPublic') }}</div>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item :label="t('node.certNode')">{{ self.name || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('node.certFingerprint')">
          <span class="fp">{{ self.fingerprint }}</span>
        </el-descriptions-item>
        <el-descriptions-item :label="t('node.certValidFrom')">{{ fmt(self.not_before) }}</el-descriptions-item>
        <el-descriptions-item :label="t('node.certValidTo')">{{ fmt(self.not_after) }}</el-descriptions-item>
        <el-descriptions-item :label="t('node.certStatus')">
          <el-tag v-if="self.status" :type="self.status === 'valid' ? 'success' : self.status === 'expired' ? 'warning' : 'danger'" size="small">{{ statusText(self.status) }}</el-tag>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item :label="t('node.certType')">{{ self.algorithm || t('node.certPublic') }}</el-descriptions-item>
        <el-descriptions-item :label="t('node.certNodeInfo')" :span="2">
          <span class="node-info">
            {{ t('node.certHostname') }}: {{ self.node_info?.hostname || '-' }}&nbsp;|&nbsp;
            {{ t('node.certIp') }}: {{ self.node_info?.ips?.join(', ') || '-' }}&nbsp;|&nbsp;
            {{ t('node.certMac') }}: {{ self.node_info?.macs?.join(', ') || '-' }}&nbsp;|&nbsp;
            {{ t('node.certOs') }}: {{ self.node_info?.os || '-' }} {{ self.node_info?.os_version || '' }}
          </span>
        </el-descriptions-item>
      </el-descriptions>
      <div class="cert-pem">
        <span>{{ t('node.certPublicTip') }}</span>
        <el-button size="small" type="primary" plain @click="downloadPem">{{ t('node.certDownload') }}</el-button>
        <el-button v-if="self.is_root" size="small" type="danger" plain :loading="rotating" @click="rotate" style="margin-left:8px">{{ t('node.certRotate') }}</el-button>
      </div>

      <!-- 总部根节点：无上级，无需私有证书 -->
      <div v-if="self.is_root" class="info-tip">
        <span class="np-dot up" />{{ t('node.certRootTip') }}
      </div>

      <template v-else>
        <el-divider />

        <!-- 私有证书（私钥）：仅本机保存，不可查看、不可导出；导入上级公有证书后生成本机私有证书 -->
        <div class="section-title">{{ t('node.certPrivate') }}</div>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="t('node.certPrivateStatus')">
            <el-tag :type="self.private_key_exists ? 'success' : 'danger'" size="small">{{ self.private_key_exists ? t('node.certPrivateReady') : t('node.certPrivateMissing') }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('node.certPrivateFp')">
            <span class="fp">{{ self.private_key_fp || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('node.certType')">{{ t('node.certPrivateType') }}</el-descriptions-item>
          <el-descriptions-item :label="t('node.certPrivateTip')" :span="2" class="node-info">{{ t('node.certPrivateTipText') }}</el-descriptions-item>
        </el-descriptions>

        <!-- 上级公有证书：上传后生成本机私有证书 -->
        <div class="parent-box">
          <div class="parent-main">
            <div class="parent-title">{{ t('node.certParentCert') }}</div>
            <template v-if="self.parent_cert_set">
              <div class="parent-ok">
                <span class="np-dot up" />{{ t('node.certParentImported') }}
              </div>
              <div class="ca-meta">{{ self.parent_cert_info }}</div>
            </template>
            <template v-else>
              <div class="parent-missing">{{ t('node.certParentNotSet') }}</div>
              <div class="ca-meta">{{ t('node.certParentTip') }}</div>
            </template>
          </div>
          <div class="parent-actions">
            <el-button size="small" @click="parentDlg=true">{{ t('node.certParentUpload') }}</el-button>
            <el-button v-if="!self.private_key_exists" size="small" type="primary" :loading="generating" @click="genPrivate">{{ t('node.certGenPrivate') }}</el-button>
            <el-button v-else size="small" type="danger" plain :loading="rotating" @click="rotate">{{ t('node.certRotate') }}</el-button>
          </div>
        </div>
      </template>
    </div>
    <el-empty v-else :description="t('node.certNotReady')" />

    <template v-if="peers.length">
      <el-divider />
      <div class="section-title">{{ t('node.certPeers') }} ({{ peers.length }})</div>
      <el-table :data="peers" size="small" class="np-table">
        <el-table-column :label="t('node.certNode')" prop="name" min-width="110" show-overflow-tooltip />
        <el-table-column :label="t('node.certNodeInfo')" min-width="180">
          <template #default="{ row }">
            <span v-if="row.node_info?.ips?.length || row.node_info?.macs?.length || row.node_info?.os" class="node-info">
              {{ t('node.certIp') }}: {{ row.node_info?.ips?.join(', ') || '-' }}<br />
              {{ t('node.certMac') }}: {{ row.node_info?.macs?.join(', ') || '-' }}<br />
              {{ t('node.certOs') }}: {{ row.node_info?.os || '-' }}
            </span>
            <span v-else class="node-info-missing">{{ t('node.certNoNodeInfo') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('node.certFingerprint')" min-width="180">
          <template #default="{ row }"><span class="fp">{{ row.fingerprint }}</span></template>
        </el-table-column>
        <el-table-column :label="t('node.certStatus')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'valid' ? 'success' : row.status === 'expired' ? 'warning' : 'danger'" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('node.certValidTo')" width="140">
          <template #default="{ row }">{{ fmt(row.not_after) }}</template>
        </el-table-column>
        <el-table-column :label="t('node.certLastVerify')" width="150">
          <template #default="{ row }">{{ fmt(row.last_verify) }}</template>
        </el-table-column>
      </el-table>
    </template>
  </div>

  <!-- 上传上级公有证书 -->
  <el-dialog v-model="parentDlg" :title="t('node.certParentUpload')" width="640px">
    <div class="dialog-tip">{{ t('node.certParentTip') }}</div>
    <el-input v-model="parentPem" type="textarea" :rows="8" :placeholder="t('node.certImportPlaceholder')" class="pem-input" />
    <el-upload :auto-upload="false" :show-file-list="false" accept=".pem,.crt,.cer,.cert,.txt" :on-change="onParentFile" style="margin:8px 0">
      <el-button size="small">{{ t('node.mqttCaUpload') }}</el-button>
    </el-upload>
    <template #footer>
      <el-button @click="parentDlg=false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="parentUploading" @click="uploadParentCert">{{ t('node.certParentUpload') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.cert-self { margin-bottom: 4px; }
.section-title { font-size: 14px; font-weight: 600; margin: 4px 0 10px; color: var(--np-text-1); }
.fp { font-family: Consolas, monospace; font-size: 12px; word-break: break-all; }
.node-info { font-size: 12px; color: var(--np-text-2); word-break: break-all; line-height: 1.7; }
.node-info-missing { font-size: 12px; color: var(--np-text-2); }
.cert-pem { margin-top: 8px; display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.cert-pem > span { font-size: 12px; color: var(--np-text-2); }

.info-tip {
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid var(--np-border);
  border-radius: 8px;
  background: var(--np-surface-1);
  font-size: 13px;
  color: var(--np-text-2);
  display: flex;
  align-items: center;
}

.parent-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-top: 12px;
  padding: 12px 14px;
  border: 1px solid var(--np-border);
  border-radius: 10px;
  background: var(--np-surface-1);
}
.parent-main { flex: 1; min-width: 0; }
.parent-title { font-weight: 600; font-size: 13px; margin-bottom: 4px; color: var(--np-text-1); }
.parent-ok { font-size: 13px; color: var(--np-success); }
.parent-missing { font-size: 13px; color: var(--np-text-2); }
.parent-actions { display: flex; flex-direction: column; gap: 6px; flex-shrink: 0; }
.ca-meta { color: var(--np-text-2); font-size: 12px; word-break: break-all; margin-top: 2px; line-height: 1.6; }

.dialog-tip { color: var(--np-text-2); font-size: 12px; margin-bottom: 8px; line-height: 1.6; }
.pem-input :deep(.el-textarea__inner) { font-family: Consolas, monospace; font-size: 12px; }
</style>
