import { useEffect, useState } from 'react'
import { api, type DiscoveredServer } from '../api/client'

export function useDiscovery(intervalMs = 3000) {
  const [found, setFound] = useState<DiscoveredServer[]>([])

  useEffect(() => {
    let cancelled = false

    const poll = async () => {
      try {
        const result = await api.discovery()
        if (!cancelled) setFound(result)
      } catch {
        // Transient mDNS/network hiccups aren't worth surfacing on every poll.
      }
    }

    void poll()
    const id = setInterval(poll, intervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [intervalMs])

  return found
}
