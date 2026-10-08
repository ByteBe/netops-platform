<script setup lang="ts">
// 脚本生成器：右栏输出
import { ElMessage } from 'element-plus'
defineProps<{ output: string; outputClean: string }>()

function copy(s: string) { navigator.clipboard?.writeText(s); ElMessage.success('已复制') }
</script>

<template>
  <div class="np-card">
    <div style="display:flex;align-items:center;margin-bottom:8px;font-weight:600">
      <span>生成结果</span><div style="flex:1"></div>
      <el-button type="primary" plain size="small" :disabled="!output" @click="copy(output)">📋 复制</el-button>
    </div>
    <pre style="background:#0b1220;color:#7dd3fc;border-radius:8px;padding:14px;font-size:12px;line-height:1.7;max-height:380px;overflow:auto;white-space:pre-wrap;margin:0">{{ output || '// 填写参数后点击"生成脚本"' }}</pre>
    <div style="display:flex;align-items:center;margin:12px 0 8px;font-weight:600">
      <span>纯净版（无注释）</span><div style="flex:1"></div>
      <el-button type="success" plain size="small" :disabled="!outputClean" @click="copy(outputClean)">📋 复制</el-button>
    </div>
    <pre style="background:#0b1220;color:#7dd3fc;border-radius:8px;padding:14px;font-size:12px;line-height:1.7;max-height:380px;overflow:auto;white-space:pre-wrap;margin:0">{{ outputClean || '// 纯净版' }}</pre>
  </div>
</template>
