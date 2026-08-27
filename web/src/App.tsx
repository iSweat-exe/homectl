import { useState } from 'react'
import { useServers } from './hooks/useServers'
import { Pairing } from './pages/Pairing'
import { ServerDetail } from './pages/ServerDetail'
import { ServerList } from './pages/ServerList'

type View =
  | { kind: 'list' }
  | { kind: 'pair'; address: string }
  | { kind: 'detail'; serverId: string }

function App() {
  const [view, setView] = useState<View>({ kind: 'list' })
  const { servers, refresh } = useServers()

  if (view.kind === 'pair') {
    return (
      <Pairing
        address={view.address}
        onDone={() => {
          void refresh()
          setView({ kind: 'list' })
        }}
        onCancel={() => setView({ kind: 'list' })}
      />
    )
  }

  if (view.kind === 'detail') {
    const server = servers.find((s) => s.id === view.serverId)
    if (!server) {
      return (
        <ServerList
          onOpenServer={(id) => setView({ kind: 'detail', serverId: id })}
          onPairAddress={(address) => setView({ kind: 'pair', address })}
        />
      )
    }
    return <ServerDetail server={server} onBack={() => setView({ kind: 'list' })} />
  }

  return (
    <ServerList
      onOpenServer={(id) => setView({ kind: 'detail', serverId: id })}
      onPairAddress={(address) => setView({ kind: 'pair', address })}
    />
  )
}

export default App
