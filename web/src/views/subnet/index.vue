<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'

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
    ElMessage.success('已复制: ' + text)
  } catch(e) {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'; ta.style.top = '0'; ta.style.left = '0'
    ta.style.width = '100px'; ta.style.height = '30px'; ta.style.opacity = '0.01'
    document.body.appendChild(ta); ta.focus(); ta.select()
    try { document.execCommand('copy'); ElMessage.success('已复制: ' + text) } catch(e2) { ElMessage.error('复制失败') }
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
  else if (firstOctet >= 224 && firstOctet < 240) cls = 'D (组播)'
  else if (firstOctet >= 240) cls = 'E (保留)'

  const isPrivate = (firstOctet === 10) ||
    (firstOctet === 172 && parseInt(ip.split('.')[1]) >= 16 && parseInt(ip.split('.')[1]) <= 31) ||
    (firstOctet === 192 && parseInt(ip.split('.')[1]) === 168)
  const isLoopback = (firstOctet === 127)
  const isLinkLocal = (firstOctet === 169 && parseInt(ip.split('.')[1]) === 254)

  let addrType = '公网地址'
  if (isLoopback) addrType = '环回地址 (127.0.0.0/8)'
  else if (isLinkLocal) addrType = '链路本地地址 (169.254.0.0/16)'
  else if (isPrivate) addrType = '私有地址 (RFC1918)'

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
    `${i + 1}. ${r.network}${r.cidr}  范围:${r.range}  广播:${r.broadcast}`
  ).join('\n')
  copy(text)
}

calc()
</script>

<template>
  <div class="ipcalc-page">
    <div class="calc-hero">
      <h2>IP 地址/子网计算器</h2>
      <p class="desc">输入 IP 地址和子网掩码前缀，自动计算网络地址、广播地址、可用主机范围</p>
      <div class="input-row">
        <el-input v-model="ipInput" placeholder="IP 地址，如 192.168.1.1" size="large" @keyup.enter="calc" />
        <span class="slash">/</span>
        <el-input-number v-model="cidrInput" :min="0" :max="32" size="large" controls-position="right" />
        <el-button type="primary" size="large" @click="calc">计 算</el-button>
      </div>
    </div>

    <div v-if="result" class="result-wrap">
      <!-- 核心结果 -->
      <div class="result-grid">
        <div class="r-card highlight">
          <div class="r-label">IP 地址</div>
          <div class="r-value">{{ result.ip }}</div>
          <div class="r-sub">/{{ result.cidr }}</div>
          <el-button size="small" class="cp-btn" @click="copy(result.ip)">复制</el-button>
        </div>
        <div class="r-card">
          <div class="r-label">子网掩码</div>
          <div class="r-value">{{ result.mask }}</div>
          <el-button size="small" class="cp-btn" @click="copy(result.mask)">复制</el-button>
        </div>
        <div class="r-card">
          <div class="r-label">网络地址</div>
          <div class="r-value hl">{{ result.network }}</div>
          <el-button size="small" class="cp-btn" @click="copy(result.network)">复制</el-button>
        </div>
        <div class="r-card">
          <div class="r-label">广播地址</div>
          <div class="r-value">{{ result.broadcast }}</div>
          <el-button size="small" class="cp-btn" @click="copy(result.broadcast)">复制</el-button>
        </div>
        <div class="r-card">
          <div class="r-label">可用主机范围</div>
          <div class="r-value sm">{{ result.firstHost }}<br>~ {{ result.lastHost }}</div>
          <el-button size="small" class="cp-btn" @click="copy(result.firstHost + ' ~ ' + result.lastHost)">复制</el-button>
        </div>
        <div class="r-card">
          <div class="r-label">可用主机数</div>
          <div class="r-value hl">{{ result.totalHosts }}</div>
          <div class="r-sub">总地址数: {{ result.totalAddrs }}</div>
        </div>
        <div class="r-card">
          <div class="r-label">反掩码</div>
          <div class="r-value sm">{{ result.wildcard }}</div>
          <el-button size="small" class="cp-btn" @click="copy(result.wildcard)">复制</el-button>
        </div>
        <div class="r-card">
          <div class="r-label">地址类别 / 类型</div>
          <div class="r-value sm">
            <span class="tag">{{ result.cls }} 类</span>
            <span class="tag blue">{{ result.type }}</span>
          </div>
        </div>
      </div>

      <!-- 扩展信息 -->
      <div class="section-card">
        <div class="section-head"><h3>IP 地址详情</h3></div>
        <div class="info-grid">
          <div class="info-row"><span class="k">十六进制</span><span class="v">{{ result.hex }}</span><el-button size="small" text type="primary" @click="copy(result.hex)">复制</el-button></div>
          <div class="info-row"><span class="k">二进制 IP</span><span class="v mono">{{ result.bin }}</span><el-button size="small" text type="primary" @click="copy(result.bin)">复制</el-button></div>
          <div class="info-row"><span class="k">二进制掩码</span><span class="v mono">{{ result.maskBin }}</span><el-button size="small" text type="primary" @click="copy(result.maskBin)">复制</el-button></div>
          <div class="info-row"><span class="k">二进制网络</span><span class="v mono hl">{{ result.netBin }}</span><el-button size="small" text type="primary" @click="copy(result.netBin)">复制</el-button></div>
        </div>
      </div>

      <!-- 子网划分 -->
      <div class="section-card">
        <div class="section-head">
          <h3>子网划分</h3>
          <div class="split-ctrl">
            <el-button size="small" type="primary" @click="copyAllSubnets">批量复制全部</el-button>
            新前缀长度
            <el-input-number v-model="splitPrefix" :min="result.cidr" :max="30" size="small" />
          </div>
        </div>
        <el-table :data="splitList" stripe max-height="280" size="small">
          <el-table-column prop="network" label="网络地址" />
          <el-table-column prop="cidr" label="CIDR" width="80" />
          <el-table-column prop="range" label="可用主机范围" />
          <el-table-column prop="broadcast" label="广播地址" />
          <el-table-column prop="hosts" label="可用数" width="80" />
          <el-table-column label="操作" width="90">
            <template #default="{ row }">
              <el-button size="small" type="primary" plain @click="copy(row.network + row.cidr + ' 范围:' + row.range + ' 广播:' + row.broadcast)">复制</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 掩码速查 -->
      <div class="section-card">
        <div class="section-head"><h3>常用子网掩码速查</h3></div>
        <div class="mask-table">
          <div class="mask-row mask-header">
            <span>CIDR</span><span>子网掩码</span><span>可用主机数</span>
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
.ipcalc-page { max-width: 1100px; margin: 0 auto; padding: 20px; }
.calc-hero {
  text-align: center; padding: 30px 0 20px;
  h2 { font-size: 26px; font-weight: 700; margin-bottom: 8px; }
  .desc { color: #94a3b8; font-size: 14px; margin-bottom: 24px; }
  .input-row {
    display: flex; gap: 8px; align-items: center; max-width: 640px; margin: 0 auto;
    .el-input { flex: 1; }
    .slash { font-size: 20px; color: #64748b; font-weight: 600; }
  }
}
.result-grid {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-bottom: 20px;
}
.r-card {
  background: #fff; border: 1px solid #e8ecf1; border-radius: 10px;
  padding: 18px; transition: all .2s; position: relative;
  &:hover { box-shadow: 0 4px 16px rgba(0,0,0,.08); border-color: #2f6bff; }
  &.highlight { border-left: 4px solid #2f6bff; }
  .r-label { font-size: 12px; color: #94a3b8; margin-bottom: 6px; }
  .cp-btn { position: absolute; top: 12px; right: 12px; font-size: 12px; padding: 2px 8px; }
  .r-value { font-size: 18px; font-weight: 700; color: #1e293b; font-family: 'Consolas', monospace; }
  .r-value.sm { font-size: 14px; display: flex; gap: 6px; flex-wrap: wrap; align-items: center; line-height: 1.5; }
  .r-value.hl { color: #2f6bff; }
  .r-sub { font-size: 12px; color: #2f6bff; margin-top: 4px; }
  .tag { font-size: 11px; padding: 2px 8px; border-radius: 4px; background: #fef3c7; color: #b45309; &.blue { background: #dbeafe; color: #1d4ed8; } }
}
.section-card {
  background: #fff; border: 1px solid #e8ecf1; border-radius: 10px;
  padding: 18px; margin-bottom: 16px;
  .section-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px;
    h3 { font-size: 15px; font-weight: 600; }
    .split-ctrl { font-size: 13px; color: #64748b; display: flex; align-items: center; gap: 8px; }
  }
}
.info-grid { display: flex; flex-direction: column; }
.info-row {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
  padding: 10px 12px; border-bottom: 1px solid #f1f5f9;
  .k { color: #64748b; font-size: 13px; width: 100px; }
  .v { flex: 1; text-align: right; }
  .v { font-family: 'Consolas', monospace; font-size: 14px; color: #1e293b; font-weight: 600; }
  .v.mono { font-size: 12px; letter-spacing: 1px; }
  .v.hl { color: #2f6bff; }
}
.mask-table { max-height: 300px; overflow-y: auto;
  .mask-row {
    display: grid; grid-template-columns: 100px 1fr 150px; padding: 8px 12px;
    font-family: 'Consolas', monospace; font-size: 13px;
    border-bottom: 1px solid #f1f5f9; cursor: pointer;
    &:hover { background: #f8fafc; }
    &.mask-header { font-weight: 600; background: #f8fafc; color: #64748b; cursor: default; }
    &.active { background: #eff6ff; color: #2f6bff; font-weight: 600; }
  }
}
</style>
