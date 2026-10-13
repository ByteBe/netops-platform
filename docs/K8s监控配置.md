# 1. 创建 Service Account 和 ClusterRoleBinding（只读权限）
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ServiceAccount
metadata:
  name: netops-monitor
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: netops-monitor
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
subjects:
- kind: ServiceAccount
  name: netops-monitor
  namespace: kube-system
EOF

# 2. 获取 Token
kubectl -n kube-system create token netops-monitor --duration=8760h

# 3. 验证（替换 <API_IP> 和 <TOKEN>）
curl -k https://<API_IP>:6443/api/v1/nodes \
  -H "Authorization: Bearer <TOKEN>"