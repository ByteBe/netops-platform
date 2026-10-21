export default {
  title: 'BGP / VPNv4 Neighbor Monitor',
  readOnly: 'Read-only via SSH, no configuration changes on devices',
  totalPeers: 'Total Peers',
  establishedPeers: 'Established',
  downPeers: 'Not Established',
  device: 'Device', peerIp: 'Peer IP', outIface: 'Out Interface', state: 'State',
  upDown: 'Up/Down', prefixRcvd: 'Prefix Rcvd', prefixSent: 'Prefix Sent',
  updatedAt: 'Updated',
  sshUser: 'SSH User', sshPass: 'SSH Password', sshPort: 'SSH Port', bgpEnable: 'Enable BGP Monitor',
  goDevice: 'Go to Device Monitoring',
  emptyHint: 'No BGP peers. Add a router in Device Monitoring and enable "BGP Monitor" with SSH credentials.',
  testBtn: 'Test Collect',
  pickDevice: 'Select device',
  noEnabledDevice: 'No device with BGP monitor enabled',
  testOk: 'Connected, found {n} peers',
  testFail: 'Test failed, check backend log',
  sshUserIpRequired: 'Please enter IP and SSH account'
}
