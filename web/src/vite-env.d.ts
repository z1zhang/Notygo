/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 'true' enables the Netlify demo build (mock data, no backend) */
  readonly VITE_DEMO_MODE?: string
}
