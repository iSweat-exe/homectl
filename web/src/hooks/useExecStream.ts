import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'

export interface ExecLine {
  stream: 'stdout' | 'stderr'
  text: string
}

type ServerMessage =
  | { type: 'stdout' | 'stderr'; data: string }
  | { type: 'exit'; code: number; exit_error?: string }
  | { type: 'error'; error: string }

function decodeBase64(data: string): string {
  return atob(data)
}

export function useExecStream(serverId: string) {
  const [lines, setLines] = useState<ExecLine[]>([])
  const [running, setRunning] = useState(false)
  const [exitCode, setExitCode] = useState<number | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  const run = useCallback(
    (command: string, args: string[] = [], workingDir = '') => {
      wsRef.current?.close()

      setLines([])
      setExitCode(null)
      setRunning(true)

      const ws = new WebSocket(api.execWsUrl(serverId))
      wsRef.current = ws

      ws.onopen = () => {
        ws.send(JSON.stringify({ type: 'start', command, args, working_dir: workingDir }))
      }
      ws.onmessage = (event) => {
        const msg = JSON.parse(event.data as string) as ServerMessage
        if (msg.type === 'stdout' || msg.type === 'stderr') {
          setLines((prev) => [...prev, { stream: msg.type, text: decodeBase64(msg.data) }])
        } else if (msg.type === 'exit') {
          setExitCode(msg.code)
          setRunning(false)
          if (msg.exit_error) {
            setLines((prev) => [...prev, { stream: 'stderr', text: msg.exit_error! }])
          }
        } else if (msg.type === 'error') {
          setLines((prev) => [...prev, { stream: 'stderr', text: msg.error }])
          setRunning(false)
        }
      }
      ws.onclose = () => setRunning(false)
      ws.onerror = () => setRunning(false)
    },
    [serverId],
  )

  const sendStdin = useCallback((text: string) => {
    wsRef.current?.send(JSON.stringify({ type: 'stdin', data: btoa(text) }))
  }, [])

  const closeStdin = useCallback(() => {
    wsRef.current?.send(JSON.stringify({ type: 'close_stdin' }))
  }, [])

  useEffect(() => {
    return () => wsRef.current?.close()
  }, [])

  return { lines, running, exitCode, run, sendStdin, closeStdin }
}
