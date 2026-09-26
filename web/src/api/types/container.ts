export interface ContainerConfig {
  docker_enabled: boolean
  docker_host: string
  k8s_enabled: boolean
  k8s_apiserver: string
  k8s_token: string
}

export interface ContainerInfo {
  id: string
  name: string
  image: string
  status: string
  cpu: number
  mem: number
}
