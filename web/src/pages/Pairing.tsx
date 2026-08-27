import { useEffect, useState } from 'react'
import { api } from '../api/client'

type Step =
  | { kind: 'connecting' }
  | { kind: 'confirm'; fingerprint: string }
  | { kind: 'pairing' }
  | { kind: 'error'; message: string }

export function Pairing({
  address,
  onDone,
  onCancel,
}: {
  address: string
  onDone: () => void
  onCancel: () => void
}) {
  const [step, setStep] = useState<Step>({ kind: 'connecting' })
  const [name, setName] = useState('')

  useEffect(() => {
    let cancelled = false
    setStep({ kind: 'connecting' })
    api
      .proposePairing(address)
      .then((res) => {
        if (!cancelled) setStep({ kind: 'confirm', fingerprint: res.fingerprint })
      })
      .catch((e: unknown) => {
        if (!cancelled) setStep({ kind: 'error', message: e instanceof Error ? e.message : String(e) })
      })
    return () => {
      cancelled = true
    }
  }, [address])

  const confirm = () => {
    if (step.kind !== 'confirm') return
    setStep({ kind: 'pairing' })
    api
      .confirmPairing(address, step.fingerprint, name)
      .then(() => onDone())
      .catch((e) => setStep({ kind: 'error', message: e instanceof Error ? e.message : String(e) }))
  }

  return (
    <div className="mx-auto flex max-w-md flex-col gap-4 p-6">
      <h1 className="text-xl font-semibold">Pair with {address}</h1>

      {step.kind === 'connecting' && (
        <div className="flex items-center gap-2">
          <span className="loading loading-spinner loading-sm" />
          Connecting…
        </div>
      )}

      {step.kind === 'error' && <div className="alert alert-error text-sm">{step.message}</div>}

      {(step.kind === 'confirm' || step.kind === 'pairing') && (
        <>
          <div className="alert alert-warning text-sm">
            Verify this fingerprint against the one shown on the server before confirming — this is
            the only thing standing between you and a spoofed daemon.
          </div>
          <div className="rounded-box bg-base-200 p-3 font-mono text-sm break-all">
            {step.kind === 'confirm' ? step.fingerprint : ''}
          </div>
          <label className="form-control">
            <span className="label-text mb-1">Name for this server</span>
            <input
              className="input input-bordered"
              value={name}
              placeholder={address}
              onChange={(e) => setName(e.target.value)}
              disabled={step.kind === 'pairing'}
            />
          </label>
          <p className="text-xs opacity-60">
            The daemon must have its pairing window open (SIGUSR1, or <code>homectl-daemon pair</code>
            {' '}run on the server) or this will be rejected.
          </p>
          <div className="flex gap-2">
            <button className="btn flex-1" onClick={onCancel} disabled={step.kind === 'pairing'}>
              Cancel
            </button>
            <button
              className="btn btn-primary flex-1"
              onClick={confirm}
              disabled={step.kind === 'pairing'}
            >
              {step.kind === 'pairing' ? 'Pairing…' : 'Confirm pairing'}
            </button>
          </div>
        </>
      )}

      {step.kind === 'error' && (
        <button className="btn" onClick={onCancel}>
          Back
        </button>
      )}
    </div>
  )
}
