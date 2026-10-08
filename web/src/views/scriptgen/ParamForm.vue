<script setup lang="ts">
// 脚本生成器：中栏参数表单
import { computed, ref, watch } from 'vue'
import type { Category, Field } from '@/types/scriptgen'

const props = defineProps<{ selected: Category | null; params: Record<string, string>; loading: boolean }>()
const emit = defineEmits<{ (e: 'gen'): void }>()
// params 是父级 reactive 对象，直接改属性即可（父级 ref 自动解包）
function set(k: string, v: string) { props.params[k] = v }
watch(() => props.selected, () => { rules.value = []; props.params.pfx_index = '10' })
watch(() => props.params.enable_prefix, (v) => { if (v === 'on' && !props.params.pfx_index) props.params.pfx_index = '10' })

const visibleFields = computed(() => {
  if (!props.selected) return []
  return props.selected.fields.filter(f => {
    if (!f.showIf) return true
    const v = props.params[f.showIf.key]
    if (f.showIf.notEq) return v !== f.showIf.notEq
    if (f.showIf.eqAny && f.showIf.eqAny.length) return f.showIf.eqAny.includes(v)
    return v === f.showIf.eq
  })
})
function required(f: Field) { return f.required }
interface Rule { idx: string; act: string; net: string; mask: string }
const rules = ref<Rule[]>([])
function addRule() {
  rules.value.push({
    idx: props.params.pfx_index || '10',
    act: props.params.pfx_action || 'permit',
    net: props.params.pfx_net || '',
    mask: props.params.pfx_masklen || ''
  })
  props.params.pfx_net = ''
  props.params.pfx_masklen = ''
  props.params.pfx_index = String(parseInt(props.params.pfx_index || '10') + 5)
}
function removeRule(i: number) { rules.value.splice(i, 1) }
function flushRules() {
  props.params.pfx_rules_multi = rules.value.map(r => `${r.idx} ${r.act} ${r.net} ${r.mask}`.trim()).join('\n')
}
</script>

<template>
  <div class="np-card">
    <div v-if="!selected" style="text-align:center;padding:80px 0;color:#909399">
      <div style="font-size:40px;margin-bottom:10px">👈</div>
      <div>从左侧选择要配置的功能</div>
    </div>
    <template v-else>
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px">
        <span style="font-size:22px">{{ selected.icon }}</span>
        <span style="font-size:15px;font-weight:600">{{ selected.name }}</span>
      </div>
      <div style="font-size:12px;color:#909399;margin-bottom:14px">{{ selected.desc }}</div>
      <el-form v-if="selected.code !== 'route_policy'" :model="params" label-position="top" size="default">
        <el-form-item v-for="f in visibleFields" :key="f.key" :label="`${f.label}${required(f) ? ' *' : ''}`">
          <el-select v-if="f.type === 'select'" :model-value="params[f.key]" style="width:100%" @update:model-value="(v: any) => set(f.key, String(v))">
            <el-option v-for="o in f.options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-switch v-else-if="f.type === 'switch'" :model-value="params[f.key]" active-value="on" inactive-value="" @update:model-value="(v: any) => set(f.key, String(v))" />
          <el-input v-else :model-value="params[f.key]" :placeholder="f.placeholder" @update:model-value="(v: string) => set(f.key, v)" />
        </el-form-item>
      </el-form>
      <div v-else>
        <el-form :model="params" label-position="top" size="default">
          <el-form-item v-for="f in visibleFields.filter(f => !['pfx_index','pfx_action','pfx_net','pfx_masklen','prefix'].includes(f.key))" :key="f.key" :label="`${f.label}${required(f) ? ' *' : ''}`">
            <el-select v-if="f.type === 'select'" :model-value="params[f.key]" style="width:100%" @update:model-value="(v: any) => set(f.key, String(v))">
              <el-option v-for="o in f.options" :key="o" :label="o" :value="o" />
            </el-select>
            <el-switch v-else-if="f.type === 'switch'" :model-value="params[f.key]" active-value="on" inactive-value="" @update:model-value="(v: any) => set(f.key, String(v))" />
            <el-input v-else :model-value="params[f.key]" :placeholder="f.placeholder" @update:model-value="(v: string) => set(f.key, v)" />
          </el-form-item>
        </el-form>
        <template v-if="params.enable_prefix === 'on'">
          <div style="display:grid;grid-template-columns:100px 120px 90px 110px 200px 80px;gap:10px;align-items:center;margin-bottom:10px;padding:10px;background:#fafbfc;border-radius:6px">
            <span style="font-size:14px;color:#303133;font-weight:500">前缀列表名</span>
            <el-input size="default" :model-value="params.prefix" placeholder="如TO-beijing" @update:model-value="(v:any)=>set('prefix',v)" />
            <el-input size="default" :model-value="params.pfx_index" placeholder="序号" @update:model-value="(v:any)=>set('pfx_index',String(v))" />
            <el-select size="default" :model-value="params.pfx_action || 'permit'" @update:model-value="(v:any)=>set('pfx_action',String(v))">
              <el-option label="permit" value="permit"/><el-option label="deny" value="deny"/>
            </el-select>
            <el-input size="default" :model-value="params.pfx_net" placeholder="匹配网段" @update:model-value="(v:any)=>set('pfx_net',v)" />
            <el-input size="default" :model-value="params.pfx_masklen" placeholder="掩码" @update:model-value="(v:any)=>set('pfx_masklen',String(v))" />
          </div>
          <div v-for="(r,i) in rules" :key="i" style="display:grid;grid-template-columns:100px 120px 90px 110px 200px 80px 40px;gap:10px;align-items:center;margin-bottom:8px;padding:8px 10px;background:#f5f7fa;border-radius:6px">
            <span style="font-size:14px;color:#303133">{{ params.prefix || '' }}</span>
            <el-input size="default" v-model="r.idx" placeholder="序号" />
            <el-select size="default" v-model="r.act">
              <el-option label="permit" value="permit"/><el-option label="deny" value="deny"/>
            </el-select>
            <el-input size="default" v-model="r.net" placeholder="匹配网段" />
            <el-input size="default" v-model="r.mask" placeholder="掩码" />
            <el-button size="small" type="danger" circle @click="removeRule(i)">🗑</el-button>
          </div>
          <el-button size="small" style="margin-bottom:10px" @click="addRule">+ 添加一行</el-button>
        </template>
      </div>
      <el-button type="success" size="large" style="width:100%" :loading="loading" @click="flushRules();emit('gen')">⚡ 生成脚本</el-button>
    </template>
  </div>
</template>
