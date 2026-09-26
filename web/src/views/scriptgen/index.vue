<script setup lang="ts">
// 脚本生成器：模板代码内置（存库、前后端不可配置），支持华为/华三，路由/交换/AC 全协议
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc } from '@/utils/request'
import { downloadText, fmtTime } from '@/utils'

const { t } = useI18n()

interface TemplateField {
  key: string
  label: string
  type: string
  required: boolean
  default: string
  options: string[]
  placeholder: string
}
interface ScriptTemplate {
  id: number
  code: string
  vendor: string
  device_type: string
  category: string
  name: string
  description: string
  schema: TemplateField[]
  enabled: boolean
}
interface CategoryNode {
  vendor: string
  types: Array<{ device_type: string; categories: string[] }>
}

const vendors = ref<Record<string, string>>({ huawei: '华为', h3c: '华三' })
const deviceTypes = ref<Record<string, string>>({ router: '路由器', switch: '交换机', ac: 'AC控制器' })
const categories = ref<CategoryNode[]>([])
const templates = ref<ScriptTemplate[]>([])
const selectedVendor = ref('huawei')

const visibleCategories = computed(() => categories.value.filter((c) => c.vendor === selectedVendor.value))

const currentTemplate = ref<ScriptTemplate | null>(null)
const fields = computed<TemplateField[]>(() => {
  if (!currentTemplate.value) return []
  return currentTemplate.value.schema || []
})

const params = ref<Record<string, string>>({})
const output = ref('')
const history = ref<Array<{ id: number; template_name: string; script: string; created_at: string }>>([])
const generating = ref(false)

async function load() {
  const [c, tmp, h] = await Promise.all([
    getEnc<CategoryNode[]>('/scriptgen/categories'),
    getEnc<ScriptTemplate[]>('/scriptgen/templates'),
    getEnc<Array<{ id: number; template_name: string; script: string; created_at: string }>>('/scriptgen/history')
  ])
  categories.value = c
  templates.value = tmp
  history.value = h
}

function selectTemplate(id: number) {
  const tmpl = templates.value.find((x) => x.id === id)
  if (!tmpl) return
  currentTemplate.value = tmpl
  params.value = {}
  fields.value.forEach((f) => {
    params.value[f.key] = f.default || ''
  })
  output.value = ''
}

async function generate() {
  if (!currentTemplate.value) return
  // 必填校验
  for (const f of fields.value) {
    if (f.required && !params.value[f.key]) {
      ElMessage.warning(`请填写：${f.label}`)
      return
    }
  }
  generating.value = true
  try {
    const r = await postEnc<{ script: string; id: number }>('/scriptgen/generate', {
      template_id: currentTemplate.value.id,
      params: params.value
    })
    output.value = r.script
    await load()
  } finally {
    generating.value = false
  }
}

function copyScript() {
  navigator.clipboard?.writeText(output.value)
  ElMessage.success(t('common.success'))
}

function downloadScript() {
  if (!output.value) return
  downloadText(`${currentTemplate.value?.code || 'script'}.txt`, output.value)
}

function selectTemplateById(vendor: string, dtype: string, category: string) {
  const tmpl = templates.value.find(
    (x) => x.vendor === vendor && x.device_type === dtype && x.category === category
  )
  if (tmpl) selectTemplate(tmpl.id)
}

function vendorLabel(v: string) {
  return vendors.value[v] || v
}
function deviceLabel(d: string) {
  return deviceTypes.value[d] || d
}

onMounted(load)
</script>

<template>
  <div class="np-page">
    <div class="np-script-layout">
      <!-- 左侧模板树 -->
      <div class="np-card np-script-tree">
        <div class="np-card-title">
          <el-icon><MagicStick /></el-icon>
          <span>{{ t('script.template') }}</span>
        </div>
        <el-select v-model="selectedVendor" style="width: 100%; margin-bottom: 12px">
          <el-option v-for="(label, key) in vendors" :key="key" :label="label" :value="key" />
        </el-select>
        <div v-for="cat in visibleCategories" :key="cat.vendor" class="np-tree-vendor">
          <div v-for="tp in cat.types" :key="tp.device_type" class="np-tree-type">
            <div class="np-tree-type-header">{{ deviceLabel(tp.device_type) }}</div>
            <div class="np-tree-cats">
              <div
                v-for="c in tp.categories" :key="c"
                class="np-cat-chip"
                :class="{ active: currentTemplate?.vendor === cat.vendor && currentTemplate?.device_type === tp.device_type && currentTemplate?.category === c }"
                @click="selectTemplateById(cat.vendor, tp.device_type, c)"
              >
                {{ c }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 中间参数表单 -->
      <div class="np-card np-script-form">
        <div class="np-card-title">
          <el-icon><EditPen /></el-icon>
          <span>{{ t('script.params') }}</span>
        </div>
        <div v-if="currentTemplate" class="np-tmpl-info">
          <span class="np-tmpl-name">{{ currentTemplate.name }}</span>
          <el-tag size="small" effect="plain">{{ vendorLabel(currentTemplate.vendor) }}</el-tag>
          <el-tag size="small" effect="plain">{{ deviceLabel(currentTemplate.device_type) }}</el-tag>
          <span class="np-tmpl-desc">{{ currentTemplate.description }}</span>
        </div>
        <div v-else class="np-empty">请选择左侧模板</div>

        <el-form v-if="currentTemplate" :model="params" label-position="top" class="np-param-form">
          <el-form-item v-for="f in fields" :key="f.key" :label="`${f.label}${f.required ? ' *' : ''}`">
            <el-select v-if="f.type === 'select'" v-model="params[f.key]" filterable allow-create style="width: 100%">
              <el-option v-for="o in f.options" :key="o" :label="o" :value="o" />
            </el-select>
            <el-input
              v-else-if="f.type === 'list'" v-model="params[f.key]" type="textarea" :rows="4"
              :placeholder="f.placeholder || '每行一条'" />
            <el-input v-else-if="f.type === 'password'" v-model="params[f.key]" type="password" show-password :placeholder="f.placeholder" />
            <el-input v-else v-model="params[f.key]" :placeholder="f.placeholder" />
          </el-form-item>
          <el-button type="primary" :loading="generating" @click="generate">
            <el-icon><MagicStick /></el-icon>{{ t('script.generate') }}
          </el-button>
        </el-form>
      </div>

      <!-- 右侧生成结果 -->
      <div class="np-card np-script-out">
        <div class="np-card-title">
          <el-icon><DocumentCopy /></el-icon>
          <span>{{ t('script.generated') }}</span>
          <div class="spacer"></div>
          <el-button size="small" text type="primary" :disabled="!output" @click="copyScript">{{ t('common.copy') }}</el-button>
          <el-button size="small" text type="primary" :disabled="!output" @click="downloadScript">{{ t('common.export') }}</el-button>
        </div>
        <pre class="np-script-pre">{{ output || '// 请选择模板并填写参数后生成' }}</pre>
      </div>
    </div>

    <!-- 生成历史 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Clock /></el-icon>
        <span>{{ t('script.history') }}</span>
      </div>
      <el-table :data="history" size="small" stripe class="np-table">
        <el-table-column prop="template_name" :label="t('script.template')" min-width="180" />
        <el-table-column prop="created_at" :label="t('common.created')" width="180">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="output = row.script">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<style lang="scss">
.np-script-layout {
  display: grid;
  grid-template-columns: 280px 1fr 1fr;
  gap: 14px;
  align-items: start;
}

.np-script-tree {
  .np-tree-vendor {
    margin-bottom: 16px;
    .np-tree-vendor-header {
      font-weight: 700;
      font-size: 14px;
      display: flex;
      align-items: center;
      gap: 6px;
      margin-bottom: 10px;
      padding-bottom: 8px;
      border-bottom: 1px solid var(--np-border);
    }
    .np-tree-type {
      padding-left: 4px;
      margin-bottom: 12px;
      .np-tree-type-header {
        font-size: 12px;
        color: var(--np-text-2);
        font-weight: 600;
        margin-bottom: 8px;
        padding-left: 8px;
        border-left: 3px solid var(--np-primary);
      }
      .np-tree-cats {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        .np-cat-chip {
          padding: 4px 10px;
          font-size: 12px;
          border-radius: 6px;
          cursor: pointer;
          border: 1px solid var(--np-border);
          background: var(--np-bg-2);
          color: var(--np-text-1);
          transition: all 0.2s;
          &:hover {
            border-color: var(--np-primary);
            color: var(--np-primary);
          }
          &.active {
            background: var(--np-primary);
            color: #fff;
            border-color: var(--np-primary);
            font-weight: 600;
          }
        }
      }
    }
  }
}

.np-tmpl-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
  .np-tmpl-name {
    font-size: 15px;
    font-weight: 600;
  }
  .np-tmpl-desc {
    margin-left: auto;
    color: var(--np-text-2);
    font-size: 12px;
  }
}

.np-param-form {
  .el-form-item {
    margin-bottom: 14px;
  }
}

.np-script-out {
  .np-script-pre {
    background: #0b1220;
    color: #7dd3fc;
    border-radius: $radius-sm;
    padding: 12px;
    font-size: 12px;
    line-height: 1.6;
    max-height: 560px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }
}

.spacer {
  flex: 1;
}
</style>
