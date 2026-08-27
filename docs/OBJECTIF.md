# OBJECTIF.md — Gestion centralisée des serveurs Linux

## Vision

Un outil léger permettant de piloter tous les serveurs Linux du réseau local
depuis un poste client unique, sans configuration manuelle, avec découverte
automatique des daemons installés sur chaque serveur.

## Objectif de la V1.0

Fournir un socle **fonctionnel minimal mais sûr** : découverte, pairing,
commandes à distance, transfert de fichiers, informations système. La V1.0
n'a pas pour but d'être complète — elle doit être **solide et sécurisée**.

## Périmètre inclus (V1.0)

### Daemon (côté serveur)

- [ ] Annonce du service en mDNS (`_homectl._tcp.local.`)
- [ ] Génération d'une paire de clés au premier démarrage + affichage de
      l'empreinte pour le pairing
- [ ] Serveur gRPC en écoute (mTLS obligatoire après pairing)
- [ ] Endpoint infos système : hostname, OS, uptime, CPU, RAM, disque
- [ ] Exécution de commande shell avec streaming stdout/stderr
- [ ] Upload et download de fichiers (par chunks)
- [ ] Packaging : binaire unique + unit systemd + script d'installation

### Client (PC)

- [ ] Scan réseau local via mDNS, liste des daemons détectés
- [ ] Flow de pairing (TOFU — trust on first use) : affichage de l'empreinte,
      confirmation manuelle, stockage de la clé du serveur
- [ ] Connexion mTLS aux serveurs pairés
- [ ] Interface web locale : le client Go embarque un serveur HTTP
      (`net/http`) exposant une API consommée par un frontend React
      (TailwindCSS + DaisyUI, graphiques Recharts), servi sur
      `localhost:PORT` — liste des serveurs, infos système, exécution de
      commande, upload/download de fichier
- [ ] Stockage local des serveurs connus (config chiffrée ou JSON restreint)

### Sécurité (non négociable pour la V1.0)

- [ ] Aucune commande exécutée sans session mTLS établie
- [ ] Pairing manuel obligatoire (pas d'auto-trust d'un nouveau daemon)
- [ ] Clés stockées avec permissions fichiers restrictives, jamais en clair
      dans un dépôt git

## Hors périmètre V1.0 (reporté explicitement)

- Dashboard graphique avancé (graphiques historiques, thèmes, temps réel
  poussé) — la V1.0 reste une page web simple, pas un tableau de bord riche
- Gestion des services systemd (start/stop/restart) — prévu en V1.1
- Monitoring / alerting / métriques historiques
- Support multi-plateforme côté serveur (Windows, macOS)
- Gestion multi-utilisateurs / permissions fines
- Mise à jour automatique du daemon (OTA)

## Stack technique retenue

| Composant           | Choix                                                        |
|----------------------|---------------------------------------------------------------|
| Langage (daemon)     | Go (binaire statique, concurrence native)                     |
| Communication        | gRPC + mTLS                                                    |
| Découverte           | mDNS (`grandcat/zeroconf` ou `hashicorp/mdns`)                 |
| Client — backend     | Serveur HTTP Go (`net/http`), sert l'API pour le frontend      |
| Client — frontend    | React + TailwindCSS + DaisyUI ; graphiques via Recharts        |

## Structure du projet

Monorepo avec séparation stricte entre daemon, client, code partagé et
frontend — chaque dossier a une seule responsabilité, ce qui permet d'ajouter
des fonctionnalités (V1.1, V1.2...) sans toucher au reste.

```
homectl/
├── cmd/
│   ├── daemon/                 # point d'entrée du daemon (main.go, flags)
│   └── client/                 # point d'entrée du client (main.go, flags)
│
├── internal/
│   ├── daemon/
│   │   ├── discovery/          # annonce mDNS du service
│   │   ├── grpcserver/         # implémentation du service gRPC
│   │   ├── system/             # collecte infos système (CPU, RAM, disque)
│   │   ├── exec/               # exécution de commandes + streaming
│   │   ├── transfer/           # upload/download de fichiers (chunks)
│   │   └── pairing/            # génération de clés, empreinte, TOFU
│   │
│   ├── client/
│   │   ├── discovery/          # scan mDNS, liste des daemons détectés
│   │   ├── grpcclient/         # wrapper client gRPC (appels aux daemons)
│   │   ├── pairing/            # confirmation TOFU, stockage clés serveurs
│   │   ├── httpapi/            # handlers HTTP consommés par le frontend
│   │   └── store/              # config locale, liste des serveurs connus
│   │
│   └── shared/
│       ├── crypto/             # helpers TLS/mTLS, génération de clés
│       ├── pb/                 # code généré depuis les fichiers .proto
│       └── config/             # structures de config communes
│
├── proto/
│   └── homectl.proto           # définition du contrat gRPC (source unique)
│
├── web/                        # frontend React, package indépendant
│   ├── src/
│   │   ├── api/                 # appels vers l'API HTTP du client Go
│   │   ├── components/          # composants UI réutilisables
│   │   ├── pages/                # écrans (liste serveurs, détail, pairing)
│   │   ├── hooks/                # logique réutilisable (polling, state)
│   │   └── App.tsx
│   ├── package.json
│   └── vite.config.ts
│
├── scripts/
│   ├── install-daemon.sh        # script d'installation sur un serveur
│   └── build.sh                 # build des binaires + du frontend
│
├── deploy/
│   └── systemd/
│       └── homectl-daemon.service
│
├── docs/
│   └── OBJECTIF.md
│
├── go.mod
└── README.md
```

**Pourquoi cette organisation :**
- `internal/daemon` et `internal/client` ne s'importent jamais l'un l'autre —
  seul `internal/shared` est commun, ce qui évite les dépendances croisées
- `proto/` est la source de vérité du contrat réseau : daemon et client
  consomment le même code généré, impossible de désynchroniser les deux
- Chaque sous-dossier (`pairing`, `exec`, `transfer`...) correspond à une
  seule responsabilité et peut évoluer ou être testé isolément
- `web/` est un package frontend totalement indépendant (son propre
  `package.json`) — remplaçable ou même détachable du repo sans impacter Go
- Ajouter une fonctionnalité V1.1 (ex. gestion systemd) = un nouveau dossier
  `internal/daemon/systemd/` sans toucher au reste de l'arborescence

## Critères de succès (Definition of Done)

1. Installation du daemon sur un serveur Linux frais en moins de 2 minutes
2. Détection automatique d'un nouveau daemon sans configuration côté client
3. Le pairing (TOFU) fonctionne et refuse toute connexion non pairée
4. Exécution d'une commande à distance avec sortie visible en direct
5. Upload et download d'un fichier vers/depuis un serveur pairé
6. Aucune donnée ne circule en clair sur le réseau

## Roadmap indicative post-V1.0

- **V1.1** — gestion des services systemd, tail de logs en direct
- **V1.2** — amélioration de l'UI web (drag-and-drop upload, thèmes),
  notifications
- **V2.0** — dashboard avec métriques historiques, alerting, gestion
  multi-utilisateurs