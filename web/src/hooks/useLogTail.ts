import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'

type ServerMessage = { type: 'line'; text: string } | { type: 'error'; error: string }

export function useLogTail(serverId: string) {
  const [lines, setLines] = useState<string[]>([])
  const [active, setActive] = useState(false)
  const [unit, setUnit] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  const stop = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null
    setActive(false)
    setUnit(null)
  }, [])

  const tail = useCallback(
    (unitName: string) => {
      wsRef.current?.close()

      setLines([])
      setActive(true)
      setUnit(unitName)

      const ws = new WebSocket(api.serviceLogsWsUrl(serverId, unitName))
      wsRef.current = ws

      ws.onmessage = (event) => {
        const msg = JSON.parse(event.data as string) as ServerMessage
        if (msg.type === 'line') {
          setLines((prev) => [...prev, msg.text])
        } else if (msg.type === 'error') {
          setLines((prev) => [...prev, `error: ${msg.error}`])
          setActive(false)
        }
      }
      ws.onclose = () => setActive(false)
      ws.onerror = () => setActive(false)
    },
    [serverId],
  )

  useEffect(() => {
    return () => wsRef.current?.close()
  }, [])

  return { lines, active, unit, tail, stop }
}
