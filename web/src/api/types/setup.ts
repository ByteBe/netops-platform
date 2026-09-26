export interface SetupDBConfig {
  type: string
  host: string
  port: number
  user: string
  password: string
  name: string
}

export interface SetupTSDBConfig {
  type: string
  host: string
  port: number
  user: string
  password: string
  db: string
}

export interface SetupAdmin {
  username: string
  password: string
  email: string
}
