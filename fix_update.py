f = r'E:\IDEA\netops-platform\web\src\views\system\index.vue'
c = open(f, encoding='utf-8').read()

# Add update tab before </el-tabs>
old_tabs = '''        </div>
      </el-tab-pane>
    </el-tabs>'''

new_tabs = '''        </div>
      </el-tab-pane>

      <!-- 系统更新 -->
      <el-tab-pane label="系统更新" name="update">
        <div class="np-card" style="padding:24px">
          <div style="margin-bottom:20px">
            <h3 style="margin:0 0 8px">当前版本</h3>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="版本号">{{ ver.version || '--' }}</el-descriptions-item>
              <el-descriptions-item label="构建时间">{{ ver.build || '--' }}</el-descriptions-item>
              <el-descriptions-item label="系统">{{ ver.os || '--' }} / {{ ver.arch || '--' }}</el-descriptions-item>
            </el-descriptions>
          </div>
          <el-divider></el-divider>
          <h3 style="margin:0 0 12px">上传更新包</h3>
          <el-upload
            :http-request="doUpload"
            :show-file-list="false"
            accept=".exe"
            drag>
            <el-icon :size="40"><UploadFilled /></el-icon>
            <div style="margin-top:8px">拖拽或点击上传新的二进制文件（.exe）</div>
            <div style="color:#999;font-size:12px;margin-top:4px">上传后将备份旧版本并自动重启</div>
          </el-upload>
          <div v-if="uploadResult" style="margin-top:16px;padding:12px;background:#f0f9ff;border-radius:6px">
            <div>文件: {{ uploadResult.filename }}</div>
            <div>大小: {{ (uploadResult.size/1024/1024).toFixed(1) }} MB</div>
            <div>校验: {{ uploadResult.sha256.substring(0,16) }}...</div>
            <el-button type="danger" style="margin-top:10px" :loading="applying" @click="doApply">确认更新并重启</el-button>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>'''

c = c.replace(old_tabs, new_tabs)

# Add script logic before onMounted
old_onmount = '''onMounted(() => {
  loadUsers()'''
new_onmount = '''// ===== 系统更新 =====
const ver = ref<any>({})
const uploadResult = ref<any>(null)
const applying = ref(false)

async function loadVersion() {
  ver.value = await getEnc('/update/version')
}

async function doUpload(opt: any) {
  const fd = new FormData()
  fd.append('file', opt.file)
  try {
    const r = await fetch('/api/v1/update/upload', {
      method: 'POST',
      headers: { 'Authorization': 'Bearer ' + localStorage.getItem('np-token') || '' },
      body: fd
    })
    const j = await r.json()
    if (j.code === 0) {
      uploadResult.value = j.data
      ElMessage.success('上传成功')
    } else {
      ElMessage.error(j.message || '上传失败')
    }
  } catch {
    ElMessage.error('上传失败')
  }
}

async function doApply() {
  applying.value = true
  try {
    await postEnc('/update/apply', { tmp_path: uploadResult.value.tmp_path })
    ElMessage.success('更新完成，系统将在几秒后重启')
    setTimeout(() => { location.reload() }, 5000)
  } finally {
    applying.value = false
  }
}

onMounted(() => {
  loadUsers()
  loadVersion()'''

c = c.replace(old_onmount, new_onmount)

open(f, 'w', encoding='utf-8').write(c)
print('OK')
