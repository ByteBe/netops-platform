<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc } from '@/utils/request'

interface Field {
  key: string; label: string; type: string; required: boolean
  placeholder: string; options: string[]
  showIf?: { key: string; eq?: string; notEq?: string; eqAny?: string[] }
}
interface Category {
  code: string; name: string; icon: string; desc: string; fields: Field[]
}
interface Group {
  type: string; label: string; items: Category[]
}

const groups = ref<Group[]>([])
const vendor = ref('huawei')
const selected = ref<Category | null>(null)
const params = ref<Record<string, string>>({})
const output = ref('')
const loading = ref(false)

const visibleFields = computed(() => {
  if (!selected.value) return []
  return selected.value.fields.filter(f => {
    if (!f.showIf) return true
    const v = params.value[f.showIf.key]
    if (f.showIf.notEq) return v !== f.showIf.notEq
    if (f.showIf.eqAny && f.showIf.eqAny.length) return f.showIf.eqAny.includes(v)
    return v === f.showIf.eq
  })
})

async function load() {
  groups.value = await getEnc<Group[]>('/scriptgen/categories')
}

function pick(c: Category) {
  selected.value = c
  params.value = {}
  c.fields.forEach(f => { params.value[f.key] = '' })
  output.value = ''
}

async function gen() {
  if (!selected.value) return
  for (const f of visibleFields.value) {
    if (f.required && !params.value[f.key]) {
      ElMessage.warning(`请填写：${f.label}`); return
    }
  }
  loading.value = true
  try {
    const r = await postEnc<{ script: string }>('/scriptgen/generate', {
      code: selected.value.code, vendor: vendor.value, params: params.value
    })
    output.value = r.script
  } finally { loading.value = false }
}

function copy() {
  if (!output.value) return
  navigator.clipboard?.writeText(output.value)
  ElMessage.success('已复制')
}

onMounted(load)
</script>

<template>
  <div class="sg-page">
    <div class="sg-toolbar">
      <span class="sg-title">⚡ 脚本生成器</span>
      <el-radio-group v-model="vendor" size="default" style="margin-left:16px">
        <el-radio-button value="huawei">华为</el-radio-button>
        <el-radio-button value="h3c">华三</el-radio-button>
        <el-radio-button value="cisco">思科</el-radio-button>
      </el-radio-group>
      <div class="spacer"></div>
      <el-button size="small" @click="output=''">清空</el-button>
    </div>

    <div class="sg-body">
      <!-- 左：功能选择 -->
      <div class="np-card sg-left">
        <div v-for="g in groups" :key="g.type" class="sg-group">
          <div class="sg-group-title">{{ g.label }}</div>
          <div class="sg-grid">
            <div v-for="c in g.items" :key="c.code"
              class="sg-item" :class="{ on: selected?.code === c.code }"
              @click="pick(c)">
              <span class="sg-ic">{{ c.icon }}</span>
              <span class="sg-lb">{{ c.name }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 中：参数 -->
      <div class="np-card sg-center">
        <div v-if="!selected" class="sg-empty">
          <div class="sg-empty-ic">👈</div>
          <div>从左侧选择要配置的功能</div>
        </div>
        <template v-else>
          <div class="sg-head">
            <span class="sg-head-icon">{{ selected.icon }}</span>
            <span class="sg-head-name">{{ selected.name }}</span>
          </div>
          <div class="sg-desc">{{ selected.desc }}</div>
          <el-form :model="params" label-position="top" size="default">
            <el-form-item v-for="f in visibleFields" :key="f.key"
              :label="`${f.label}${f.required ? ' *' : ''}`">
              <el-select v-if="f.type === 'select'" v-model="params[f.key]" style="width:100%">
                <el-option v-for="o in f.options" :key="o" :label="o" :value="o" />
              </el-select>
              <el-input v-else-if="f.type === 'password'" v-model="params[f.key]" type="password" show-password :placeholder="f.placeholder" />
              <el-input v-else v-model="params[f.key]" :placeholder="f.placeholder" />
            </el-form-item>
          </el-form>
          <el-button type="success" size="large" class="sg-btn" :loading="loading" @click="gen">
            ⚡ 生成脚本
          </el-button>
        </template>
      </div>

      <!-- 右：输出 -->
      <div class="np-card sg-right">
        <div class="sg-out-head">
          <span>生成结果</span>
          <div class="spacer"></div>
          <el-button type="primary" plain size="small" :disabled="!output" @click="copy">📋 复制</el-button>
        </div>
        <pre class="sg-out">{{ output || '// 填写参数后点击"生成脚本"' }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sg-page { display: flex; flex-direction: column; gap: 12px; }
.sg-toolbar { display: flex; align-items: center; }
.sg-title { font-size: 16px; font-weight: 700; }
.spacer { flex: 1; }
.sg-body { display: grid; grid-template-columns: 220px 1fr 1fr; gap: 12px; align-items: start; }
.sg-left { max-height: 620px; overflow-y: auto; }
.sg-group { margin-bottom: 14px; }
.sg-group-title { font-size: 12px; color: #909399; font-weight: 600; margin-bottom: 8px; padding-left: 8px; border-left: 3px solid #67c23a; }
.sg-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.sg-item {
  display: flex; flex-direction: column; align-items: center; gap: 4px;
  padding: 10px 4px; border-radius: 8px; cursor: pointer;
  border: 1px solid #e4e7ed; background: #fafafa; transition: all .2s;
}
.sg-item:hover { border-color: #67c23a; }
.sg-item.on { background: #67c23a; border-color: #67c23a; }
.sg-item.on .sg-lb { color: #fff; }
.sg-ic { font-size: 22px; }
.sg-lb { font-size: 11px; text-align: center; line-height: 1.3; }
.sg-empty { text-align: center; padding: 80px 0; color: #909399; }
.sg-empty-ic { font-size: 40px; margin-bottom: 10px; }
.sg-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.sg-head-icon { font-size: 22px; }
.sg-head-name { font-size: 15px; font-weight: 600; }
.sg-desc { font-size: 12px; color: #909399; margin-bottom: 14px; }
.sg-btn { width: 100%; margin-top: 8px; }
.sg-out-head { display: flex; align-items: center; margin-bottom: 8px; font-weight: 600; }
.sg-out {
  background: #0b1220; color: #7dd3fc; border-radius: 8px;
  padding: 14px; font-size: 12px; line-height: 1.7;
  max-height: 560px; overflow: auto; white-space: pre-wrap;
}
</style>



