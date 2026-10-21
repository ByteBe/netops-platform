<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc } from '@/utils/request'
import type { Category, Group } from '@/types/scriptgen'
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

async function load() { groups.value = await getEnc<Group[]>('/scriptgen/categories') }
function pick(c: Category) {
  selected.value = c; params.value = {}
  c.fields.forEach(f => { params.value[f.key] = '' })
  output.value = ''; outputClean.value = ''
}
async function gen() {
  if (!selected.value) return
  for (const f of selected.value.fields) {
    if (!f.required) continue
    // 检查 showIf
    if (f.showIf) {
      const v = params.value[f.showIf.key]
      if (f.showIf.notEq && v === f.showIf.notEq) continue
      if (f.showIf.eqAny && f.showIf.eqAny.length && !f.showIf.eqAny.includes(v)) continue
      if (f.showIf.eq && v !== f.showIf.eq) continue
    }
    if (!params.value[f.key]) { ElMessage.warning(`${t('script.fillRequired')}：${f.label}`); return }
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
        <ParamForm :selected="selected" :params="params" :loading="loading" @gen="gen" />
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
