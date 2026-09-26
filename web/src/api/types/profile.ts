export interface ProfileInfo {
  id: number
  username: string
  email: string
  employee_no: string
  role: string
}

export interface ChangePwdReq {
  old_password: string
  new_password: string
}
