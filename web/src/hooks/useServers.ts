import { useCallback, useEffect, useState } from 'react'
import { api, type KnownServer } from '../api/client'

export function useServers() {
  const [servers, setServers] = useState<KnownServer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      setServers(await api.servers())
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return { servers, loading, error, refresh }
}
