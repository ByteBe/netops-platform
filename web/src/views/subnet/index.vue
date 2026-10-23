<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const ipInput = ref('192.168.1.1')
const cidrInput = ref(24)
const result = ref<any>(null)

function ipToInt(ip: string): number {
  const parts = ip.split('.').map(Number)
  if (parts.length !== 4 || parts.some(p => isNaN(p) || p < 0 || p > 255)) return -1
  return ((parts[0] << 24) >>> 0) + (parts[1] << 16) + (parts[2] << 8) + parts[3]
}
function intToIp(n: number): string {
  return [(n >>> 24) & 255, (n >>> 16) & 255, (n >>> 8) & 255, n & 255].join('.')
}
function toHex(ip: string): string {
  return '0x' + ip.split('.').map(p => parseInt(p).toString(16).padStart(2, '0').toUpperCase()).join('')
}
function toBinary(ip: string): string {
  return ip.split('.').map(p => parseInt(p).toString(2).padStart(8, '0')).join('.')
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(t('subnet.copied') + text)
  } catch(e) {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'; ta.style.top = '0'; ta.style.left = '0'
    ta.style.width = '100px'; ta.style.height = '30px'; ta.style.opacity = '0.01'
    document.body.appendChild(ta); ta.focus(); ta.select()
    try { document.execCommand('copy'); ElMessage.success(t('subnet.copied') + text) } catch(e2) { ElMessage.error(t('subnet.copyFailed')) }
    document.body.removeChild(ta)
  }
}

function calc() {
  const ip = ipInput.value.trim()
  const cidr = parseInt(String(cidrInput.value))
  const ipInt = ipToInt(ip)
  if (ipInt < 0 || cidr < 0 || cidr > 32) { result.value = null; return }

  const maskInt = cidr === 0 ? 0 : (0xFFFFFFFF << (32 - cidr)) >>> 0
  const networkInt = (ipInt & maskInt) >>> 0
  const broadcastInt = (networkInt | (~maskInt >>> 0)) >>> 0
  const firstHost = cidr >= 31 ? networkInt : (networkInt + 1) >>> 0
  const lastHost = cidr >= 31 ? broadcastInt : (broadcastInt - 1) >>> 0
  const totalAddrs = Math.pow(2, 32 - cidr)
  const totalHosts = cidr >= 31 ? (cidr === 31 ? 2 : 1) : totalAddrs - 2

  const firstOctet = parseInt(ip.split('.')[0])
  let cls = 'A'
  if (firstOctet >= 128 && firstOctet < 192) cls = 'B'
  else if (firstOctet >= 192 && firstOctet < 224) cls = 'C'
  else if (firstOctet >= 224 && firstOctet < 240) cls = t('subnet.multicast')
  else if (firstOctet >= 240) cls = t('subnet.reserved')

  const isPrivate = (firstOctet === 10) ||
    (firstOctet === 172 && parseInt(ip.split('.')[1]) >= 16 && parseInt(ip.split('.')[1]) <= 31) ||
    (firstOctet === 192 && parseInt(ip.split('.')[1]) === 168)
  const isLoopback = (firstOctet === 127)
  const isLinkLocal = (firstOctet === 169 && parseInt(ip.split('.')[1]) === 254)

  let addrType = t('subnet.public')
  if (isLoopback) addrType = t('subnet.loopback')
  else if (isLinkLocal) addrType = t('subnet.linkLocal')
  else if (isPrivate) addrType = t('subnet.private')

  result.value = {
    ip, cidr,
    network: intToIp(networkInt),
    broadcast: intToIp(broadcastInt),
    mask: intToIp(maskInt),
    wildcard: intToIp(~maskInt >>> 0),
    firstHost: intToIp(firstHost),
    lastHost: intToIp(lastHost),
    totalHosts: totalHosts.toLocaleString(),
    totalAddrs: totalAddrs.toLocaleString(),
    cls, type: addrType,
    hex: toHex(ip),
    int: ipInt.toLocaleString(),
    bin: toBinary(ip),
    netBin: toBinary(intToIp(networkInt)),
    maskBin: toBinary(intToIp(maskInt))
  }
}

const splitPrefix = ref(26)
const splitList = computed(() => {
  if (!result.value) return []
  const netInt = ipToInt(result.value.network)
  const newBits = splitPrefix.value - result.value.cidr
  if (newBits < 0) return []
  const subnets = Math.pow(2, newBits)
  const size = Math.pow(2, 32 - splitPrefix.value)
  const list: any[] = []
  for (let i = 0; i < Math.min(subnets, 100); i++) {
    const sn = (netInt + i * size) >>> 0
    const bc = (sn + size - 1) >>> 0
    list.push({
      cidr: `/${splitPrefix.value}`,
      network: intToIp(sn),
      range: splitPrefix.value >= 31 ? '-' : `${intToIp(sn + 1)} ~ ${intToIp(bc - 1)}`,
      broadcast: intToIp(bc),
      hosts: size - 2
    })
  }
  return list
})

const commonMasks = computed(() => {
  const list = []
  for (let c = 8; c <= 30; c++) {
    const mask = c === 0 ? 0 : (0xFFFFFFFF << (32 - c)) >>> 0
    list.push({ cidr: `/${c}`, mask: intToIp(mask), hosts: (Math.pow(2, 32 - c) - 2).toLocaleString() })
  }
  return list
})

function copyAllSubnets() {
  const text = splitList.value.map((r: any, i: number) =>
    `${i + 1}. ${r.network}${r.cidr}  ${t('subnet.hostRange')}:${r.range}  ${t('subnet.broadcast')}:${r.broadcast}`
  ).join('\n')
  copy(text)
}

calc()
</script>

<template>
  <div class="np-page ipcalc-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('subnet.title') }}</div>
        <div class="np-page-head-desc">{{ t('subnet.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-input v-model="ipInput" :placeholder="t('subnet.ipPlaceholder')" class="calc-ip-input" @keyup.enter="calc" />
        <span class="slash">/</span>
        <el-input-number v-model="cidrInput" :min="0" :max="32" controls-position="right" />
        <el-button type="primary" @click="calc">{{ t('subnet.calc') }}</el-button>
      </div>
    </div>

    <div v-if="result" class="result-wrap">
      <!-- 核心结果 -->
      <div class="np-kpi-grid result-grid">
        <div class="np-kpi r-card">
          <div class="np-kpi-icon"><el-icon><InfoFilled /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value">{{ result.ip }}<span class="slash24">/{{ result.cidr }}</span></div>
            <div class="np-kpi-label">{{ t('subnet.ipAddr') }}</div>
          </div>
          <el-button size="small" class="cp-btn" @click="copy(result.ip + '/' + result.cidr)">{{ t('common.copy') }}</el-button>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-info"><el-icon><Key /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value">{{ result.mask }}</div>
            <div class="np-kpi-label">{{ t('subnet.mask') }}</div>
          </div>
          <el-button size="small" class="cp-btn" @click="copy(result.mask)">{{ t('common.copy') }}</el-button>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value hl">{{ result.network }}</div>
            <div class="np-kpi-label">{{ t('subnet.network') }}</div>
          </div>
          <el-button size="small" class="cp-btn" @click="copy(result.network)">{{ t('common.copy') }}</el-button>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-warning"><el-icon><Promotion /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value">{{ result.broadcast }}</div>
            <div class="np-kpi-label">{{ t('subnet.broadcast') }}</div>
          </div>
          <el-button size="small" class="cp-btn" @click="copy(result.broadcast)">{{ t('common.copy') }}</el-button>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-info"><el-icon><Position /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value sm">{{ result.firstHost }}</div>
            <div class="np-kpi-sub">~ {{ result.lastHost }}</div>
            <div class="np-kpi-label">{{ t('subnet.hostRange') }}</div>
          </div>
          <el-button size="small" class="cp-btn" @click="copy(result.firstHost + ' ~ ' + result.lastHost)">{{ t('common.copy') }}</el-button>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-success"><el-icon><Coin /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value hl">{{ result.totalHosts }}</div>
            <div class="np-kpi-sub">{{ t('subnet.totalAddrs') }}: {{ result.totalAddrs }}</div>
            <div class="np-kpi-label">{{ t('subnet.hostCount') }}</div>
          </div>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-info"><el-icon><Grid /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value sm">{{ result.wildcard }}</div>
            <div class="np-kpi-label">{{ t('subnet.wildcard') }}</div>
          </div>
          <el-button size="small" class="cp-btn" @click="copy(result.wildcard)">{{ t('common.copy') }}</el-button>
        </div>
        <div class="np-kpi r-card">
          <div class="np-kpi-icon is-warning"><el-icon><Collection /></el-icon></div>
          <div class="np-kpi-body">
            <div class="np-kpi-value r-value sm">
              <span class="tag">{{ result.cls }} {{ t('subnet.class') }}</span>
              <span class="tag blue">{{ result.type }}</span>
            </div>
            <div class="np-kpi-label">{{ t('subnet.classType') }}</div>
          </div>
        </div>
      </div>

      <!-- 扩展信息 -->
      <div class="np-card section-card">
        <div class="np-card-title"><el-icon><Document /></el-icon><span>{{ t('subnet.detail') }}</span></div>
        <div class="info-grid">
          <div class="info-row"><span class="k">{{ t('subnet.hex') }}</span><span class="v">{{ result.hex }}</span><el-button size="small" text type="primary" @click="copy(result.hex)">{{ t('common.copy') }}</el-button></div>
          <div class="info-row"><span class="k">{{ t('subnet.binIp') }}</span><span class="v mono">{{ result.bin }}</span><el-button size="small" text type="primary" @click="copy(result.bin)">{{ t('common.copy') }}</el-button></div>
          <div class="info-row"><span class="k">{{ t('subnet.binMask') }}</span><span class="v mono">{{ result.maskBin }}</span><el-button size="small" text type="primary" @click="copy(result.maskBin)">{{ t('common.copy') }}</el-button></div>
          <div class="info-row"><span class="k">{{ t('subnet.binNet') }}</span><span class="v mono hl">{{ result.netBin }}</span><el-button size="small" text type="primary" @click="copy(result.netBin)">{{ t('common.copy') }}</el-button></div>
        </div>
      </div>

      <!-- 子网划分 -->
      <div class="np-card section-card">
        <div class="np-card-title">
          <el-icon><Share /></el-icon>
          <span>{{ t('subnet.split') }}</span>
          <div class="spacer"></div>
          <div class="split-ctrl">
            {{ t('subnet.newPrefix') }}
            <el-input-number v-model="splitPrefix" :min="result.cidr" :max="30" size="small" />
            <el-button size="small" type="primary" @click="copyAllSubnets">{{ t('subnet.copyAll') }}</el-button>
          </div>
        </div>
        <el-table :data="splitList" stripe max-height="280" size="small" class="np-table">
          <el-table-column prop="network" :label="t('subnet.network')" />
          <el-table-column prop="cidr" label="CIDR" width="80" />
          <el-table-column prop="range" :label="t('subnet.hostRange')" />
          <el-table-column prop="broadcast" :label="t('subnet.broadcast')" />
          <el-table-column prop="hosts" :label="t('subnet.hosts')" width="80" />
          <el-table-column :label="t('subnet.actions')" width="90">
            <template #default="{ row }">
              <el-button size="small" type="primary" plain @click="copy(row.network + row.cidr + ' ' + row.range + ' ' + row.broadcast)">{{ t('common.copy') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 掩码速查 -->
      <div class="np-card section-card">
        <div class="np-card-title"><el-icon><Grid /></el-icon><span>{{ t('subnet.commonMasks') }}</span></div>
        <div class="mask-table">
          <div class="mask-row mask-header">
            <span>CIDR</span><span>{{ t('subnet.mask') }}</span><span>{{ t('subnet.hostCount') }}</span>
          </div>
          <div class="mask-row" v-for="m in commonMasks" :key="m.cidr"
               :class="{active: m.cidr === '/' + result.cidr}" @click="cidrInput = parseInt(m.cidr.slice(1)); calc()">
            <span>{{ m.cidr }}</span><span>{{ m.mask }}</span><span>{{ m.hosts }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.ipcalc-page {
  max-width: 1100px;
  margin: 0 auto;
  width: 100%;
}
.calc-ip-input { width: 260px; }
.slash { font-size: 20px; color: var(--np-text-2); font-weight: 600; }

.result-grid {
  margin-bottom: 4px;
  grid-template-columns: repeat(auto-fill, minmax(215px, 1fr));
}

.r-card {
  .cp-btn { position: absolute; top: 12px; right: 12px; font-size: 12px; padding: 2px 8px; }
  .np-kpi-icon { display: none; } // 计算结果卡隐藏装饰图标，避免挤压数值
  .np-kpi-body { padding-right: 46px; } // 为右上角复制按钮让位，避免遮挡数值
  .r-value {
    font-family: 'Consolas', monospace;
    font-size: 14px;
    font-weight: 700;
    color: var(--np-text-1);
    white-space: nowrap;
    overflow: hidden;
    .slash24 { font-size: 12px; font-weight: 600; opacity: .8; }
  }
  .r-value.sm { font-size: 14px; display: flex; gap: 6px; flex-wrap: wrap; align-items: center; line-height: 1.5; }
  .r-value.hl { color: var(--np-text-1); }
  .tag {
    font-size: 11px; padding: 2px 8px; border-radius: 4px;
    background: var(--np-warning-weak); color: var(--np-warning);
    font-weight: 600;
    &.blue { background: var(--np-info-weak); color: var(--np-info); }
  }
}

.section-card { margin-bottom: 16px; }
.split-ctrl { font-size: 13px; color: var(--np-text-2); display: flex; align-items: center; gap: 8px; }

.info-grid { display: flex; flex-direction: column; }
.info-row {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
  padding: 10px 12px; border-bottom: 1px solid var(--np-border);
  .k { color: var(--np-text-2); font-size: 13px; width: 100px; }
  .v { flex: 1; text-align: right; }
  .v { font-family: 'Consolas', monospace; font-size: 14px; color: var(--np-text-1); font-weight: 600; }
  .v.mono { font-size: 12px; letter-spacing: 1px; }
  .v.hl { color: var(--np-text-1); font-weight: 700; }
}
.mask-table { max-height: 300px; overflow-y: auto;
  .mask-row {
    display: grid; grid-template-columns: 100px 1fr 150px; padding: 8px 12px;
    font-family: 'Consolas', monospace; font-size: 13px;
    font-variant-numeric: tabular-nums;
    border-bottom: 1px solid var(--np-border); cursor: pointer;
    span:last-child { text-align: right; }
    &:hover { background: var(--np-hover); }
    &.mask-header { font-weight: 600; background: var(--np-hover); color: var(--np-text-2); cursor: default; }
    &.active { background: var(--np-primary-weak); color: var(--np-primary); font-weight: 600; }
  }
}
</style>
