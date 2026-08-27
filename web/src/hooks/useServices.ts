import { useCallback, useEffect, useState } from 'react'
import { api, type ServiceUnit } from '../api/client'

export function useServices(serverId: string | null, intervalMs = 4000) {
  const [services, setServices] = useState<ServiceUnit[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!serverId) return

    let cancelled = false

    const poll = async () => {
      try {
        const result = await api.services(serverId)
        if (!cancelled) {
          setServices(result)
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

  const refresh = useCallback(() => {
    if (!serverId) return
    api
      .services(serverId)
      .then(setServices)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [serverId])

  return { services, error, refresh }
}
