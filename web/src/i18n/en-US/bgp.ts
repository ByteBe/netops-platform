export default {
  title: 'BGP / VPNv4 Neighbor Monitor',
  readOnly: 'Read-only via SSH, no configuration changes on devices',
  device: 'Device', peerIp: 'Peer IP', state: 'State',
  upDown: 'Up/Down', prefixRcvd: 'Prefix Rcvd', prefixSent: 'Prefix Sent',
  updatedAt: 'Updated',
  sshUser: 'SSH User', sshPass: 'SSH Password', sshPort: 'SSH Port', bgpEnable: 'Enable BGP Monitor',
  goDevice: 'Go to Device Monitoring',
  emptyHint: 'No BGP peers. Add a router in Device Monitoring and enable "BGP Monitor" with SSH credentials.'
}
