/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

// sm-crypto 无类型声明，补充模块声明
declare module 'sm-crypto' {
  export const sm2: {
    doEncrypt(msg: string | Uint8Array | number[], publicKey: string, cipherMode?: 0 | 1): string
    doDecrypt(encryptData: string, privateKey: string, cipherMode?: 0 | 1): string
    generateKeyPairHex(): { privateKey: string; publicKey: string }
  }
  export const sm3: {
    sm3(input: string | Uint8Array): string
  }
  export const sm4: {
    encrypt(inArray: number[] | string | Uint8Array, key: number[] | string, options?: {
      mode?: 'cbc' | 'ecb'
      iv?: number[] | string
      output?: 'array' | 'string' | 'hex'
    }): number[] | string
    decrypt(inArray: number[] | string | Uint8Array, key: number[] | string, options?: {
      mode?: 'cbc' | 'ecb'
      iv?: number[] | string
      output?: 'array' | 'string' | 'hex'
    }): number[] | string
  }
}
