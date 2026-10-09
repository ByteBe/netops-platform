<script setup lang="ts">
// IP 地址管理：子网/已用未用/使用登记/SSH 下发绑定（可选开关）
import { ref, reactive, onMounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { fmtTime } from '@/utils'

const { t } = useI18n()

interface Subnet {
  id: number
  name: string
  cidr: string
  gateway: string
  vlan: number
  group_id: number
  binding_enabled: boolean
  description: string
  created_at: string
}
interface IPItem {
  ip: string
  status: 'used' | 'unused'
  owner_name: string
  mac: string
  office: string
  record_id: number
  bind_device_id: number
  bind_status: string
  bind_log: string
  remark: string
}
interface BindDevice {
  id: number
  name: string
  ip: string
  ssh_user: string
  ssh_port: number
  auth_type: string
  credential: string
  vendor: string
  enable: boolean
  remark: string
}

const subnets = ref<Subnet[]>([])
const currentSubnet = ref<Subnet | null>(null)
const ipItems = ref<IPItem[]>([])
const ipTotal = ref(0)
const ipPage = ref(1)
const ipStatus = ref<'all' | 'used' | 'unused'>('all')
const ipKeyword = ref('')
const bindingEnable = ref(false)

const subnetDialog = ref(false)
const subnetForm = reactive({ id: 0, name: '', cidr: '', gateway: '', vlan: 0, binding_enabled: false, description: '' })

const recDialog = ref(false)
const recForm = reactive({ subnet_id: 0, ip: '', owner_name: '', mac: '', office: '', remark: '' })
const editRecordId = ref(0)

const bindDialog = ref(false)
const bindDevices = ref<BindDevice[]>([])
const bindForm = reactive({
  id: 0, name: '', ip: '', ssh_user: '', ssh_port: 22, auth_type: 'password',
  credential: '', vendor: 'huawei', enable: true, remark: ''
})
const testOut = ref('')
const arpDialog = ref(false)
const arpDeviceId = ref(0)

async function loadSubnets() {
  subnets.value = await getEnc<Subnet[]>('/ipam/subnets')
  if (!currentSubnet.value && subnets.value.length) {
    currentSubnet.value = subnets.value[0]
  }
}

async function loadIps() {
  if (!currentSubnet.value) return
  const r = await getEnc<{ total: number; list: IPItem[]; subnet: Subnet }>(
    `/ipam/subnets/${currentSubnet.value.id}/ips?status=${ipStatus.value}&page=${ipPage.value}&size=20&keyword=${encodeURIComponent(ipKeyword.value)}`
  )
  ipTotal.value = r.total
  ipItems.value = r.list
}

async function loadBindDevices() {
  bindDevices.value = await getEnc<BindDevice[]>('/ipam/bind-devices')
}

async function loadSettings() {
  const s = await getEnc<{ ipam_binding_enable: boolean }>('/ipam/settings')
  bindingEnable.value = s.ipam_binding_enable
}

async function selectSubnet(s: Subnet) {
  currentSubnet.value = s
  ipPage.value = 1
  await loadIps()
}

async function saveSubnet() {
  if (!subnetForm.name || !subnetForm.cidr) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (subnetForm.id) {
    await putEnc(`/ipam/subnets/${subnetForm.id}`, subnetForm)
  } else {
    await postEnc('/ipam/subnets', subnetForm)
  }
  ElMessage.success(t('common.success'))
  subnetDialog.value = false
  await loadSubnets()
}
async function removeSubnet(s: Subnet) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/ipam/subnets/${s.id}`)
  if (currentSubnet.value?.id === s.id) currentSubnet.value = null
  ElMessage.success(t('common.success'))
  await loadSubnets()
}

function openRec(item: IPItem) {
  editRecordId.value = item.record_id || 0
  Object.assign(recForm, {
    subnet_id: currentSubnet.value!.id,
    ip: item.ip,
    owner_name: item.owner_name,
    mac: item.mac,
    office: item.office,
    remark: item.remark || ''
  })
  recDialog.value = true
}
async function saveRec() {
  if (!recForm.owner_name) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (editRecordId.value) {
    await putEnc(`/ipam/records/${editRecordId.value}`, {
      owner_name: recForm.owner_name, mac: recForm.mac, office: recForm.office, remark: recForm.remark
    })
  } else {
    await postEnc('/ipam/records', recForm)
  }
  ElMessage.success(t('common.success'))
  recDialog.value = false
  await loadIps()
}
async function unuse(item: IPItem) {
  await ElMessageBox.confirm(`${t('ipam.releaseConfirm')} ${item.ip} ?`, t('common.tip'), { type: 'warning' })
  await delEnc(`/ipam/records/${item.record_id}`)
  ElMessage.success(t('common.success'))
  await loadIps()
}

async function bind(item: IPItem) {
  if (!bindingEnable.value) {
    ElMessage.warning(t('ipam.bindingNotEnabled'))
    return
  }
  if (!bindDevices.value.length) {
    ElMessage.warning(t('ipam.addSwitchFirst'))
    return
  }
  const { value } = await ElMessageBox.prompt(t('ipam.selectSwitch'), t('ipam.bindDelivery'), {
    inputValue: String(bindDevices.value[0].id),
    inputPlaceholder: bindDevices.value.map((d) => `${d.id}:${d.name}`).join('，'),
    confirmButtonText: t('common.confirm'),
    cancelButtonText: t('common.cancel')
  })
  await postEnc(`/ipam/records/${item.record_id}/bind`, { bind_device_id: Number(value) })
  ElMessage.success(t('common.success'))
  await loadIps()
}

function openBindDevice(d?: BindDevice) {
  testOut.value = ''
  if (d) {
    Object.assign(bindForm, d)
  } else {
    Object.assign(bindForm, {
      id: 0, name: '', ip: '', ssh_user: '', ssh_port: 22, auth_type: 'password',
      credential: '', vendor: 'huawei', enable: true, remark: ''
    })
  }
  bindDialog.value = true
}
async function saveBindDevice() {
  if (!bindForm.name || !bindForm.ip) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (bindForm.id) {
    await putEnc(`/ipam/bind-devices/${bindForm.id}`, bindForm)
  } else {
    await postEnc('/ipam/bind-devices', bindForm)
  }
  ElMessage.success(t('common.success'))
  bindDialog.value = false
  await loadBindDevices()
}
async function removeBindDevice(d: BindDevice) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/ipam/bind-devices/${d.id}`)
  ElMessage.success(t('common.success'))
  await loadBindDevices()
}
async function testBindDevice(d: BindDevice) {
  try {
    const r = await postEnc<{ ok: boolean; output: string }>(`/ipam/bind-devices/${d.id}/test`, {})
    testOut.value = r.output
    ElMessage.success(t('ipam.sshTestOk'))
  } catch (e: any) {
    testOut.value = e?.message || String(e)
    ElMessage.error(t('ipam.sshTestFail'))
  }
}
async function toggleBinding() {
  await putEnc('/ipam/settings', { ipam_binding_enable: bindingEnable.value })
  ElMessage.success(t('common.success'))
}

async function importARP() {
  if (!currentSubnet.value) return
  if (!bindDevices.value.length) {
    ElMessage.warning(t('ipam.addBindSwitchFirst'))
    return
  }
  arpDeviceId.value = bindDevices.value[0].id
  arpDialog.value = true
}

async function confirmImportArp() {
  if (!currentSubnet.value || !arpDeviceId.value) return
  arpDialog.value = false
  const r = await postEnc<{ imported: number; updated: number; arp_count: number }>(
    `/ipam/subnets/${currentSubnet.value.id}/import-arp`, { bind_device_id: arpDeviceId.value })
  ElMessage.success(`${t('ipam.importDone')}：${t('ipam.newCount')} ${r.imported}，${t('ipam.updatedCount')} ${r.updated}，${t('ipam.arpCount')} ${r.arp_count}`)
  await loadIps()
}
function rowClassName(data: { row: IPItem; rowIndex: number }): string {
  return data.row.status === 'used' ? '' : 'row-unused'
}

onMounted(async () => {
  await loadSettings()
  await loadSubnets()
  await loadBindDevices()
  if (currentSubnet.value) await loadIps()
})
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <span class="np-page-desc">{{ t('ipam.pageDesc') }}</span>
      <div class="spacer"></div>
      <el-switch v-model="bindingEnable" @change="toggleBinding" :active-text="t('ipam.bindingEnabled')" />
      <el-button type="primary" @click="openBindDevice()">
        <el-icon><Connection /></el-icon>{{ t('ipam.bindDevice') }}
      </el-button>
      <el-button type="primary" @click="subnetDialog = true">
        <el-icon><Plus /></el-icon>{{ t('ipam.addSubnet') }}
      </el-button>
    </div>

    <!-- 子网列表 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><MapLocation /></el-icon>
        <span>{{ t('ipam.subnets') }}</span>
      </div>
      <div class="np-subnet-tabs">
        <div
          v-for="s in subnets" :key="s.id" class="np-subnet-tab"
          :class="{ active: currentSubnet?.id === s.id }"
          @click="selectSubnet(s)"
        >
          <div class="np-st-name">{{ s.name }}</div>
          <div class="np-st-cidr">{{ s.cidr }} <el-tag v-if="s.binding_enabled" size="small" type="success">{{ t('ipam.bound') }}</el-tag></div>
          <div class="np-st-ops">
            <el-button size="small" text type="primary" @click.stop="Object.assign(subnetForm, s); subnetDialog = true">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click.stop="removeSubnet(s)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
        <div v-if="!subnets.length" class="np-empty">{{ t('common.noData') }}</div>
      </div>
    </div>

    <!-- IP 列表 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Position /></el-icon>
        <span>{{ currentSubnet?.name || t('ipam.subnets') }} · {{ t('ipam.ipList') }}（{{ ipTotal }}）</span>
        <div class="spacer"></div>
        <el-radio-group v-model="ipStatus" @change="ipPage = 1; loadIps()" size="small">
          <el-radio-button value="all">{{ t('ipam.all') }}</el-radio-button>
          <el-radio-button value="used">{{ t('ipam.used') }}</el-radio-button>
          <el-radio-button value="unused">{{ t('ipam.unused') }}</el-radio-button>
        </el-radio-group>
        <el-input v-model="ipKeyword" :placeholder="t('ipam.searchIp')" size="small" style="width: 200px; margin-left: 10px" clearable @change="ipPage = 1; loadIps()" />
        <el-button size="small" type="warning" style="margin-left: 10px" :disabled="!bindDevices.length" @click="importARP">
          <el-icon><Download /></el-icon> {{ t('ipam.readArp') }}
        </el-button>
      </div>
      <el-table :data="ipItems" stripe size="small" class="np-table" :row-class-name="rowClassName">
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column :label="t('ipam.status')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'used' ? 'danger' : 'success'">
              {{ row.status === 'used' ? t('ipam.used') : t('ipam.unused') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="owner_name" :label="t('ipam.owner')" width="120" />
        <el-table-column prop="mac" :label="t('ipam.mac')" width="150" />
        <el-table-column prop="office" :label="t('ipam.office')" width="140" />
        <el-table-column :label="t('ipam.bindStatus')" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.bind_status === 'ok'" size="small" type="success">{{ t('ipam.boundTag') }}</el-tag>
            <el-tag v-else-if="row.bind_status === 'fail'" size="small" type="danger">{{ t('ipam.failTag') }}</el-tag>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'unused'" size="small" text type="primary" @click="openRec(row)">{{ t('ipam.register') }}</el-button>
            <template v-else>
              <el-button size="small" text type="primary" @click="openRec(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" text type="primary" :disabled="!bindingEnable" @click="bind(row)">{{ t('ipam.bindDeliver') }}</el-button>
              <el-button size="small" text type="danger" @click="unuse(row)">{{ t('ipam.release') }}</el-button>
            </template>
          </template>
        </el-table-column>
      </el-table>
      <div class="np-pager">
        <el-pagination background layout="prev, pager, next, total" :total="ipTotal" :page-size="20" v-model:current-page="ipPage" @current-change="loadIps" />
      </div>
    </div>

    <!-- 绑定配置设备 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Connection /></el-icon>
        <span>{{ t('ipam.bindSwitchTitle') }}</span>
        <div class="spacer"></div>
        <el-button size="small" type="primary" @click="openBindDevice()">{{ t('ipam.add') }}</el-button>
      </div>
      <el-table :data="bindDevices" stripe size="small" class="np-table">
        <el-table-column prop="name" :label="t('ipam.name')" min-width="140" />
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="ssh_user" :label="t('ipam.sshUser')" width="100" />
        <el-table-column prop="ssh_port" :label="t('ipam.port')" width="80" />
        <el-table-column prop="vendor" :label="t('ipam.vendor')" width="80" />
        <el-table-column :label="t('common.actions')" width="200">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="testBindDevice(row)">{{ t('ipam.test') }}</el-button>
            <el-button size="small" text type="primary" @click="openBindDevice(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="removeBindDevice(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="testOut" class="np-test-out">{{ testOut }}</div>
    </div>

    <!-- 导入ARP对话框 -->
    <el-dialog v-model="arpDialog" :title="t('ipam.importArp')" width="420px">
      <el-form label-width="100px">
        <el-form-item :label="t('ipam.selectSwitchLabel')">
          <el-select v-model="arpDeviceId" style="width:100%">
            <el-option v-for="d in bindDevices" :key="d.id" :label="`${d.name} (${d.ip})`" :value="d.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="arpDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmImportArp">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 子网对话框 -->
    <el-dialog v-model="subnetDialog" :title="subnetForm.id ? t('common.edit') : t('ipam.addSubnet')" width="480px">
      <el-form :model="subnetForm" label-width="110px">
        <el-form-item :label="t('common.name')" required>
          <el-input v-model="subnetForm.name" />
        </el-form-item>
        <el-form-item label="CIDR" required>
          <el-input v-model="subnetForm.cidr" placeholder="10.10.0.0/24" />
        </el-form-item>
        <el-form-item :label="t('ipam.gateway')">
          <el-input v-model="subnetForm.gateway" />
        </el-form-item>
        <el-form-item label="VLAN">
          <el-input-number v-model="subnetForm.vlan" :min="0" :max="4094" />
        </el-form-item>
        <el-form-item :label="t('ipam.bindingEnabled')">
          <el-switch v-model="subnetForm.binding_enabled" />
        </el-form-item>
        <el-form-item :label="t('common.remark')">
          <el-input v-model="subnetForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subnetDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveSubnet">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 登记对话框 -->
    <el-dialog v-model="recDialog" :title="editRecordId ? t('common.edit') : t('ipam.register')" width="440px">
      <el-form :model="recForm" label-width="110px">
        <el-form-item label="IP">
          <el-input :model-value="recForm.ip" disabled />
        </el-form-item>
        <el-form-item :label="t('ipam.owner')" required>
          <el-input v-model="recForm.owner_name" />
        </el-form-item>
        <el-form-item :label="t('ipam.mac')">
          <el-input v-model="recForm.mac" placeholder="AA-BB-CC-DD-EE-FF" />
        </el-form-item>
        <el-form-item :label="t('ipam.office')">
          <el-input v-model="recForm.office" />
        </el-form-item>
        <el-form-item :label="t('common.remark')">
          <el-input v-model="recForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="recDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveRec">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 绑定交换机对话框 -->
    <el-dialog v-model="bindDialog" :title="bindForm.id ? t('common.edit') : t('ipam.addBindSwitch')" width="500px">
      <el-form :model="bindForm" label-width="110px">
        <el-form-item :label="t('common.name')" required>
          <el-input v-model="bindForm.name" />
        </el-form-item>
        <el-form-item label="IP" required>
          <el-input v-model="bindForm.ip" />
        </el-form-item>
        <el-form-item :label="t('ipam.sshUser')">
          <el-input v-model="bindForm.ssh_user" />
        </el-form-item>
        <el-form-item :label="t('ipam.sshPort')">
          <el-input-number v-model="bindForm.ssh_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item :label="t('ipam.authType')">
          <el-select v-model="bindForm.auth_type">
            <el-option :label="t('ipam.password')" value="password" />
            <el-option :label="t('ipam.key')" value="key" />
          </el-select>
        </el-form-item>
        <el-form-item :label="bindForm.auth_type === 'key' ? t('ipam.privateKey') : t('ipam.password')">
          <el-input v-if="bindForm.auth_type === 'key'" v-model="bindForm.credential" type="textarea" :rows="4" />
          <el-input v-else v-model="bindForm.credential" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('ipam.vendor')">
          <el-select v-model="bindForm.vendor">
            <el-option :label="t('ipam.huawei')" value="huawei" />
            <el-option :label="t('ipam.h3c')" value="h3c" />
            <el-option :label="t('ipam.cisco')" value="cisco" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bindDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveBindDevice">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-subnet-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;

  .np-subnet-tab {
    border: 1px solid var(--np-border);
    border-radius: $radius-sm;
    padding: 10px 14px;
    min-width: 180px;
    cursor: pointer;
    transition: all 0.2s;

    &.active {
      border-color: var(--np-primary);
      background: var(--np-primary-weak);
    }

    .np-st-name {
      font-weight: 600;
      font-size: 14px;
    }
    .np-st-cidr {
      color: var(--np-text-2);
      font-size: 12px;
      margin: 2px 0;
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .np-st-ops {
      display: flex;
      justify-content: flex-end;
    }
  }
}

.row-unused {
  color: var(--np-text-2);
}

.np-pager {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.np-test-out {
  margin-top: 10px;
  background: #0b1220;
  color: #7dd3fc;
  border-radius: $radius-sm;
  padding: 10px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 180px;
  overflow: auto;
}
</style>
