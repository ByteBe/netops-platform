# K8s 部署步骤

## 1. 构建镜像

```bash
cd deploy/k8s
docker build -t registry.example.com/netops:v2.2.0 .
docker push registry.example.com/netops:v2.2.0
```

## 2. 前置依赖（MySQL / TDengine）

K8s 内可用 Helm 装 MySQL：
```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install mysql bitnami/mysql -n netops --set auth.database=netops
helm install tdengine tdengine/tdengine -n netops
```

或直接用现有外部 MySQL/TDengine，改 yaml 里的 env。

## 3. 部署

```bash
# 改 secret 里的密码
kubectl apply -f netops-k8s.yaml

# 看状态
kubectl -n netops get pods,svc
kubectl -n netops logs -f deploy/netops
```

## 4. 访问

`http://节点IP:30821`

如需外网暴露，把 Service type 改为 LoadBalancer 或加 Ingress。
