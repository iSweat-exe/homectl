import { useEffect, useRef, useState } from 'react'
import { api, type KnownServer, type ServiceActionName } from '../api/client'
import { Gauge } from '../components/Gauge'
import { useExecStream } from '../hooks/useExecStream'
import { useLogTail } from '../hooks/useLogTail'
import { useServices } from '../hooks/useServices'
import { useSystemInfo } from '../hooks/useSystemInfo'
import { formatBytes, formatUptime } from '../lib/format'

export function ServerDetail({ server, onBack }: { server: KnownServer; onBack: () => void }) {
  const { info, error } = useSystemInfo(server.id)

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-8 p-6">
      <header className="flex items-center gap-3">
        <button className="btn btn-ghost btn-sm" onClick={onBack}>
          ← Back
        </button>
        <h1 className="text-xl font-semibold">{server.name}</h1>
        <span className="badge badge-ghost">{server.address}</span>
      </header>

      {error && <div className="alert alert-error text-sm">{error}</div>}

      {info && (
        <section className="flex flex-col gap-4">
          <div className="text-sm opacity-70">
            {info.hostname} · {info.os} {info.platform_version} · up {formatUptime(info.uptime_seconds)}
          </div>
          <div className="flex flex-wrap justify-center gap-6">
            <Gauge label="CPU" percent={info.cpu_usage_percent} detail={info.cpu_model} />
            <Gauge
              label="RAM"
              percent={(info.mem_used_bytes / Math.max(info.mem_total_bytes, 1)) * 100}
              detail={`${formatBytes(info.mem_used_bytes)} / ${formatBytes(info.mem_total_bytes)}`}
            />
            <Gauge
              label="Disk"
              percent={(info.disk_used_bytes / Math.max(info.disk_total_bytes, 1)) * 100}
              detail={`${formatBytes(info.disk_used_bytes)} / ${formatBytes(info.disk_total_bytes)}`}
            />
          </div>
        </section>
      )}

      <ServicesPanel serverId={server.id} />
      <ExecConsole serverId={server.id} />
      <FileTransfer serverId={server.id} />
    </div>
  )
}

function statusBadgeClass(activeState: string): string {
  if (activeState === 'active') return 'badge badge-success'
  if (activeState === 'failed') return 'badge badge-error'
  return 'badge badge-ghost'
}

function ServicesPanel({ serverId }: { serverId: string }) {
  const { services, error, refresh } = useServices(serverId)
  const { lines, active, unit: tailedUnit, tail, stop } = useLogTail(serverId)
  const [filter, setFilter] = useState('')
  const [pending, setPending] = useState<string | null>(null)
  const logRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
  }, [lines])

  const doAction = async (unitName: string, action: ServiceActionName) => {
    setPending(`${unitName}:${action}`)
    try {
      await api.serviceAction(serverId, unitName, action)
    } finally {
      setPending(null)
      refresh()
    }
  }

  const filtered = (services ?? []).filter((s) => s.name.toLowerCase().includes(filter.toLowerCase()))

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-sm font-medium opacity-70">Services</h2>
      {error && <div className="alert alert-error text-sm">{error}</div>}

      <input
        className="input input-bordered input-sm"
        placeholder="Filter services…"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
      />

      <div className="max-h-96 overflow-y-auto rounded-box border border-base-300">
        <table className="table table-sm">
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Boot</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((svc) => {
              const enabled = svc.unit_file_state === 'enabled'
              return (
                <tr key={svc.name}>
                  <td className="font-mono text-xs">{svc.name}</td>
                  <td>
                    <span className={statusBadgeClass(svc.active_state)}>{svc.active_state}</span>
                    <span className="ml-1 text-xs opacity-60">{svc.sub_state}</span>
                  </td>
                  <td>
                    <span className={`badge ${enabled ? 'badge-outline' : 'badge-ghost'}`}>
                      {svc.unit_file_state}
                    </span>
                  </td>
                  <td>
                    <div className="flex flex-wrap gap-1">
                      <button
                        className="btn btn-xs"
                        disabled={pending === `${svc.name}:start`}
                        onClick={() => void doAction(svc.name, 'start')}
                      >
                        Start
                      </button>
                      <button
                        className="btn btn-xs"
                        disabled={pending === `${svc.name}:stop`}
                        onClick={() => void doAction(svc.name, 'stop')}
                      >
                        Stop
                      </button>
                      <button
                        className="btn btn-xs"
                        disabled={pending === `${svc.name}:restart`}
                        onClick={() => void doAction(svc.name, 'restart')}
                      >
                        Restart
                      </button>
                      <button
                        className="btn btn-xs"
                        disabled={pending === `${svc.name}:${enabled ? 'disable' : 'enable'}`}
                        onClick={() => void doAction(svc.name, enabled ? 'disable' : 'enable')}
                      >
                        {enabled ? 'Disable' : 'Enable'}
                      </button>
                      <button className="btn btn-xs btn-outline" onClick={() => tail(svc.name)}>
                        Logs
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
            {services !== null && filtered.length === 0 && (
              <tr>
                <td colSpan={4} className="text-center text-xs opacity-50">
                  No services match.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {tailedUnit && (
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2 text-sm opacity-70">
            <span>
              Logs — <span className="font-mono">{tailedUnit}</span>
            </span>
            {active && <span className="badge badge-success badge-xs">live</span>}
            <button className="btn btn-xs btn-outline ml-auto" onClick={stop}>
              Stop tailing
            </button>
          </div>
          <div
            ref={logRef}
            className="h-56 overflow-y-auto rounded-box bg-neutral p-3 font-mono text-xs text-neutral-content"
          >
            {lines.length === 0 && <span className="opacity-50">Waiting for output…</span>}
            {lines.map((line, i) => (
              <div key={i}>{line}</div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}

function ExecConsole({ serverId }: { serverId: string }) {
  const { lines, running, exitCode, run, sendStdin, closeStdin } = useExecStream(serverId)
  const [command, setCommand] = useState('')
  const [stdinText, setStdinText] = useState('')
  const logRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
  }, [lines])

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-sm font-medium opacity-70">Run command</h2>
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          if (command.trim()) run(command.trim())
        }}
      >
        <input
          className="input input-bordered flex-1 font-mono"
          placeholder="uptime"
          value={command}
          onChange={(e) => setCommand(e.target.value)}
          disabled={running}
        />
        <button type="submit" className="btn btn-primary" disabled={running || !command.trim()}>
          {running ? 'Running…' : 'Run'}
        </button>
      </form>

      <div
        ref={logRef}
        className="h-56 overflow-y-auto rounded-box bg-neutral p-3 font-mono text-xs text-neutral-content"
      >
        {lines.length === 0 && <span className="opacity-50">No output yet.</span>}
        {lines.map((l, i) => (
          <div key={i} className={l.stream === 'stderr' ? 'text-error' : undefined}>
            {l.text}
          </div>
        ))}
        {exitCode !== null && <div className="mt-1 opacity-60">exit code {exitCode}</div>}
      </div>

      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          sendStdin(stdinText + '\n')
          setStdinText('')
        }}
      >
        <input
          className="input input-bordered flex-1 font-mono"
          placeholder="stdin"
          value={stdinText}
          onChange={(e) => setStdinText(e.target.value)}
          disabled={!running}
        />
        <button type="submit" className="btn btn-outline" disabled={!running}>
          Send
        </button>
        <button type="button" className="btn btn-outline" disabled={!running} onClick={closeStdin}>
          Close stdin
        </button>
      </form>
    </section>
  )
}

function FileTransfer({ serverId }: { serverId: string }) {
  const [uploadPath, setUploadPath] = useState('')
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadStatus, setUploadStatus] = useState<string | null>(null)
  const [downloadPath, setDownloadPath] = useState('')

  const doUpload = async () => {
    if (!uploadFile || !uploadPath) return
    setUploadStatus('Uploading…')
    try {
      const summary = await api.upload(serverId, uploadPath, uploadFile)
      setUploadStatus(`Wrote ${summary.bytes_written} bytes to ${summary.path}`)
    } catch (e) {
      setUploadStatus(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-sm font-medium opacity-70">File transfer</h2>

      <div className="flex flex-col gap-2 rounded-box border border-base-300 p-3">
        <span className="text-sm font-medium">Upload</span>
        <div className="flex gap-2">
          <input
            type="file"
            className="file-input file-input-bordered flex-1"
            onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
          />
          <input
            className="input input-bordered flex-1 font-mono"
            placeholder="/absolute/destination/path"
            value={uploadPath}
            onChange={(e) => setUploadPath(e.target.value)}
          />
          <button className="btn btn-outline" onClick={() => void doUpload()} disabled={!uploadFile || !uploadPath}>
            Upload
          </button>
        </div>
        {uploadStatus && <span className="text-xs opacity-70">{uploadStatus}</span>}
      </div>

      <div className="flex flex-col gap-2 rounded-box border border-base-300 p-3">
        <span className="text-sm font-medium">Download</span>
        <div className="flex gap-2">
          <input
            className="input input-bordered flex-1 font-mono"
            placeholder="/absolute/source/path"
            value={downloadPath}
            onChange={(e) => setDownloadPath(e.target.value)}
          />
          <a
            className={`btn btn-outline ${downloadPath ? '' : 'btn-disabled'}`}
            href={downloadPath ? api.downloadUrl(serverId, downloadPath) : undefined}
          >
            Download
          </a>
        </div>
      </div>
    </section>
  )
}
