export default {
  title: 'BGP / VPNv4 邻居监控',
  readOnly: '通过 SSH 只读采集，不对设备做任何配置修改',
  device: '设备', peerIp: '邻居 IP', state: '状态',
  upDown: 'Up/Down', prefixRcvd: '收到 Prefix', prefixSent: '发布 Prefix',
  updatedAt: '采集时间',
  sshUser: 'SSH账号', sshPass: 'SSH密码', sshPort: 'SSH端口', bgpEnable: '启用BGP监控',
  goDevice: '前往设备监控添加',
  emptyHint: '暂无 BGP 邻居。请先到「设备监控」纳管路由器，填写 SSH 账号密码并开启"启用BGP监控"。'
}
