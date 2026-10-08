export interface Field {
  key: string; label: string; type: string; required: boolean
  placeholder: string; options: string[]
  showIf?: { key: string; eq?: string; notEq?: string; eqAny?: string[] }
}
export interface Category { code: string; name: string; icon: string; desc: string; fields: Field[] }
export interface Group { type: string; label: string; items: Category[] }
