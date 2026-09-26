export interface LoginForm {
  username: string
  password: string
}

export interface ChangePwdForm {
  oldPassword: string
  newPassword: string
  confirm: string
}

export interface LoginResult {
  token: string
  username: string
  email: string
  employee_no: string
  role: string
  must_change_pwd: boolean
}
