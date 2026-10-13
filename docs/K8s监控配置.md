# Kubernetes 监控配置

NetOps 平台通过 Kubernetes API Server 采集 Pod、节点状态。

## 一、创建只读 ServiceAccount

```bash
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
```

## 二、获取 Token（有效期 1 年）

```bash
kubectl -n kube-system create token netops-monitor --duration=8760h
```

## 三、验证

```bash
curl -k https://<API_IP>:6443/api/v1/nodes \
  -H "Authorization: Bearer <TOKEN>"
```

## 四、在 NetOps 平台添加

1. 进入「容器监控」→「添加 K8s 集群」
2. API Server 地址：`https://集群IP:6443`
3. Token 填入上面获取的 token
4. 勾选"跳过 TLS 验证"（自签名证书时）
5. 保存后自动采集 Pod 状态
