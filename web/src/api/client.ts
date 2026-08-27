export interface DiscoveredServer {
  instance_name: string
  address: string
  paired: boolean
  server_id?: string
}

export interface KnownServer {
  id: string
  name: string
  address: string
  fingerprint: string
  paired_at: string
}

export interface SystemInfo {
  hostname: string
  os: string
  platform_version: string
  uptime_seconds: number
  cpu_model: string
  cpu_cores: number
  cpu_usage_percent: number
  mem_total_bytes: number
  mem_used_bytes: number
  disk_total_bytes: number
  disk_used_bytes: number
}

export interface ProposeResponse {
  address: string
  fingerprint: string
}

export interface UploadSummary {
  bytes_written: number
  path: string
}

export interface ServiceUnit {
  name: string
  description: string
  load_state: string
  active_state: string
  sub_state: string
  unit_file_state: string
}

export type ServiceActionName = 'start' | 'stop' | 'restart' | 'enable' | 'disable'

export interface ServiceActionResponse {
  success: boolean
  message: string
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}) as { error?: string })
    throw new Error(body.error ?? `${res.status} ${res.statusText}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  discovery: () => req<DiscoveredServer[]>('/api/discovery'),

  servers: () => req<KnownServer[]>('/api/servers'),
  deleteServer: (id: string) => req<void>(`/api/servers/${id}`, { method: 'DELETE' }),
  systemInfo: (id: string) => req<SystemInfo>(`/api/servers/${id}/system-info`),

  proposePairing: (address: string) =>
    req<ProposeResponse>('/api/pairing/propose', {
      method: 'POST',
      body: JSON.stringify({ address }),
    }),
  confirmPairing: (address: string, fingerprint: string, name: string) =>
    req<KnownServer>('/api/pairing/confirm', {
      method: 'POST',
      body: JSON.stringify({ address, fingerprint, name }),
    }),

  upload: async (id: string, path: string, file: File): Promise<UploadSummary> => {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(`/api/servers/${id}/upload?path=${encodeURIComponent(path)}`, {
      method: 'POST',
      body: form,
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}) as { error?: string })
      throw new Error(body.error ?? `${res.status} ${res.statusText}`)
    }
    return res.json()
  },
  downloadUrl: (id: string, path: string) =>
    `/api/servers/${id}/download?path=${encodeURIComponent(path)}`,

  execWsUrl: (id: string) => {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${window.location.host}/api/servers/${id}/exec`
  },

  services: (id: string) => req<ServiceUnit[]>(`/api/servers/${id}/services`),
  serviceAction: (id: string, unit: string, action: ServiceActionName) =>
    req<ServiceActionResponse>(`/api/servers/${id}/services/${encodeURIComponent(unit)}/action`, {
      method: 'POST',
      body: JSON.stringify({ action }),
    }),
  serviceLogsWsUrl: (id: string, unit: string) => {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${window.location.host}/api/servers/${id}/services/${encodeURIComponent(unit)}/logs`
  },
}
