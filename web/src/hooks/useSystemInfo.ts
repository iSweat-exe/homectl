import { useEffect, useState } from 'react'
import { api, type SystemInfo } from '../api/client'

export function useSystemInfo(serverId: string | null, intervalMs = 3000) {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!serverId) return

    let cancelled = false

    const poll = async () => {
      try {
        const result = await api.systemInfo(serverId)
        if (!cancelled) {
          setInfo(result)
          setError(null)
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      }
    }

    void poll()
    const id = setInterval(poll, intervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [serverId, intervalMs])

  return { info, error }
}
