import { useDiscovery } from '../hooks/useDiscovery'
import { useServers } from '../hooks/useServers'

export function ServerList({
  onOpenServer,
  onPairAddress,
}: {
  onOpenServer: (id: string) => void
  onPairAddress: (address: string) => void
}) {
  const { servers, loading, error, refresh } = useServers()
  const discovered = useDiscovery()

  const unpaired = discovered.filter((d) => !d.paired)

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-8 p-6">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">homectl</h1>
        <button className="btn btn-ghost btn-sm" onClick={() => void refresh()}>
          Refresh
        </button>
      </header>

      {error && <div className="alert alert-error text-sm">{error}</div>}

      <section>
        <h2 className="mb-2 text-sm font-medium opacity-70">Paired servers</h2>
        {loading ? (
          <span className="loading loading-spinner loading-sm" />
        ) : servers.length === 0 ? (
          <p className="text-sm opacity-60">No servers paired yet.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {servers.map((s) => (
              <li key={s.id}>
                <button
                  className="btn btn-outline w-full justify-start"
                  onClick={() => onOpenServer(s.id)}
                >
                  <span className="font-medium">{s.name}</span>
                  <span className="opacity-60">{s.address}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="mb-2 text-sm font-medium opacity-70">Discovered on this network</h2>
        {unpaired.length === 0 ? (
          <p className="text-sm opacity-60">No new daemons found via mDNS.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {unpaired.map((d) => (
              <li key={d.address} className="flex items-center justify-between rounded-box border border-base-300 p-3">
                <div>
                  <div className="font-medium">{d.instance_name}</div>
                  <div className="text-sm opacity-60">{d.address}</div>
                </div>
                <button className="btn btn-primary btn-sm" onClick={() => onPairAddress(d.address)}>
                  Pair
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="mb-2 text-sm font-medium opacity-70">Manual address</h2>
        <ManualAddressForm onSubmit={onPairAddress} />
      </section>
    </div>
  )
}

function ManualAddressForm({ onSubmit }: { onSubmit: (address: string) => void }) {
  return (
    <form
      className="flex gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        const form = e.currentTarget
        const input = form.elements.namedItem('address') as HTMLInputElement
        if (input.value.trim()) onSubmit(input.value.trim())
      }}
    >
      <input
        name="address"
        placeholder="192.168.1.42:47331"
        className="input input-bordered flex-1"
      />
      <button type="submit" className="btn btn-outline">
        Connect
      </button>
    </form>
  )
}
