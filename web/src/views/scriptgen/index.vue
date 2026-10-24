<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc } from '@/utils/request'
import type { Category, Field, Group } from '@/types/scriptgen'
import CateList from './CateList.vue'
import ParamForm from './ParamForm.vue'
import ScriptOutput from './ScriptOutput.vue'
const { t } = useI18n()

const groups = ref<Group[]>([])
const vendor = ref('huawei')
const selected = ref<Category | null>(null)
const params = ref<Record<string, string>>({})
const output = ref('')
const outputClean = ref('')
const loading = ref(false)
const errors = ref<Record<string, string>>({})

async function load() { groups.value = await getEnc<Group[]>('/scriptgen/categories') }
function pick(c: Category) {
  selected.value = c; params.value = {}
  c.fields.forEach(f => { params.value[f.key] = '' })
  output.value = ''; outputClean.value = ''; errors.value = {}
}

// ---------- 输入格式校验（与后端同一套规则） ----------
function visible(f: Field, m: Record<string, string>) {
  if (!f.showIf) return true
  const v = m[f.showIf.key]
  if (f.showIf.notEq) return v !== f.showIf.notEq
  if (f.showIf.eqAny && f.showIf.eqAny.length) return f.showIf.eqAny.includes(v)
  return v === f.showIf.eq
}
function validIP(s: string): boolean {
  const t = s.trim()
  if (!/^\d{1,3}(\.\d{1,3}){3}$/.test(t)) return false
  return t.split('.').every(x => +x <= 255)
}
function validMask(s: string): boolean {
  if (!validIP(s)) return false
  const n = s.split('.').reduce((a, x) => (a * 256 + +x) >>> 0, 0) >>> 0
  if (n === 0) return true
  // 数出 n 中 1 的个数（与后端 bits.OnesCount32 一致，不能用 clz32：0xFFFFFF00 最高位为 1 时 clz32 为 0）
  let ones = 0, t = n
  while (t) { ones += t & 1; t >>>= 1 }
  const expect = (0xFFFFFFFF << (32 - ones)) >>> 0
  return n === expect
}
const macRe = /^(?:[0-9A-Fa-f]{4}\.){2}[0-9A-Fa-f]{4}$|^(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$/
function validateField(f: Field, code: string, raw: string): string {
  const v = raw.trim()
  if (!v) return f.required ? t('script.validate.empty') : ''
  switch (f.type) {
    case 'ip':
      if (!validIP(v)) return t('script.validate.ip')
      break
    case 'network': {
      const p = v.split('/')
      if (p.length !== 2 || !validIP(p[0])) return t('script.validate.network')
      const n = +p[1]
      if (!/^\d+$/.test(p[1].trim()) || n < 0 || n > 32) return t('script.validate.prefix')
      break
    }
    case 'number':
      if (!/^\d+$/.test(v)) return t('script.validate.number')
      break
    case 'mask':
      if (!validMask(v)) return t('script.validate.mask')
      break
    case 'password':
      if (v.length < 6) return t('script.validate.passlen')
      break
  }
  switch (f.key) {
    case 'peer_ip': case 'vpn4_enable_peer': case 'vpn4_adv_comm': case 'vpn4_next_hop_inv': case 'dns':
      if (v.split(',').some(x => !validIP(x))) return t('script.validate.iplist')
      break
    case 'peer_as':
      if (v.split(',').some(x => !/^\d+$/.test(x.trim()))) return t('script.validate.aslist')
      break
    case 'vlan': case 'vlans':
      if (v.split(',').some(x => x.trim().split('-').some(n => !/^\d+$/.test(n.trim())))) return t('script.validate.vlan')
      break
    case 'vty': {
      const p = v.split(/\s+/)
      if (p.length !== 2 || !p.every(x => /^\d+$/.test(x))) return t('script.validate.vty')
      break
    }
    case 'srcmac':
      if (!macRe.test(v)) return t('script.validate.mac')
      break
    case 'ip':
      if (code === 'mpls_vpn') {
        const p = v.split(/\s+/)
        if (p.length !== 2 || !validIP(p[0]) || !validMask(p[1])) return t('script.validate.ipmask')
      }
      break
    case 'pfx_net':
      if (!validIP(v)) return t('script.validate.ip')
      break
  }
  return ''
}

async function gen() {
  if (!selected.value) return
  errors.value = {}
  for (const f of selected.value.fields) {
    if (!visible(f, params.value)) continue
    const err = validateField(f, selected.value.code, params.value[f.key] || '')
    if (err) { errors.value[f.key] = err; return }
  }
  loading.value = true
  try {
    const r = await postEnc<{ script: string }>('/scriptgen/generate', {
      code: selected.value.code, vendor: vendor.value, params: params.value
    })
    output.value = r.script
    outputClean.value = r.script.split('\n').map((l: string) => l.replace(/\s*\/\/.*$/, '')).filter((l: string) => l.trim()).join('\n')
  } finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="np-page sg-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">⚡ {{ t('script.scriptGen') }}</div>
        <div class="np-page-head-desc">{{ t('script.pageDesc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-radio-group v-model="vendor" size="default">
          <el-radio-button value="huawei">{{ t('script.huawei') }}</el-radio-button>
          <el-radio-button value="h3c">{{ t('script.h3c') }}</el-radio-button>
          <el-radio-button value="cisco">{{ t('script.cisco') }}</el-radio-button>
        </el-radio-group>
        <el-button size="small" @click="output='';outputClean=''">{{ t('script.clear') }}</el-button>
      </div>
    </div>
    <div class="sg-grid">
      <CateList :groups="groups" :selected="selected" @pick="pick" />
      <div class="sg-right">
        <ParamForm :selected="selected" :params="params" :loading="loading" :errors="errors" @gen="gen" />
        <ScriptOutput :output="output" :output-clean="outputClean" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.sg-grid { display: grid; grid-template-columns: 220px 1fr; gap: 16px; align-items: start; }
.sg-right { display: flex; flex-direction: column; gap: 16px; }
@media (max-width: 900px) {
  .sg-grid { grid-template-columns: 1fr; }
}
</style>
