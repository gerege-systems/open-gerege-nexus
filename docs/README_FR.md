# Gerege Nexus

**Plateforme intégrée d'opérations numériques**

**Gerege Nexus** est une plateforme modulaire open source qui relie les
services, les opérations, les systèmes et les données des organisations
publiques et privées. Elle place le **mongol au premier plan** et s'intègre
directement à l'infrastructure numérique nationale de la Mongolie (DAN, E-ID,
XYP / ХУР).

*Nexus* désigne le point de connexion : là où se rejoignent organisations,
services, processus, systèmes, utilisateurs et données. La plateforme
elle-même n'est liée à aucun secteur — ce sont les modules qui y tournent qui
donnent son caractère à un déploiement.

Les modules sont compilés dans un seul binaire Go, tandis qu'un magasin
d'applications adossé à PostgreSQL décide des applications actives pour chaque
locataire — une séparation modulaire sans les appels réseau ni le coût
d'exploitation des microservices.

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_AR.md"><img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> العربية</a>
  &nbsp;·&nbsp;
  <a href="README_ZH.md"><img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> 中文</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> <b>Français</b>
  &nbsp;·&nbsp;
  <a href="README_RU.md"><img src="assets/icons/flag-ru.png" width="18" height="18" alt=""> Русский</a>
  &nbsp;·&nbsp;
  <a href="README_ES.md"><img src="assets/icons/flag-es.png" width="18" height="18" alt=""> Español</a>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16.3-black.svg)](https://nextjs.org)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../CONTRIBUTING.md)

---

## Sommaire

- [Auteurs](#auteurs)
- [Capacités principales](#capacités-principales)
- [Applications métier](#applications-métier)
- [Structure du dépôt](#structure-du-dépôt)
- [Démarrage](#démarrage)
- [Configuration](#configuration)
- [Aperçu de l'API](#aperçu-de-lapi)
- [Tests et contrôles qualité](#tests-et-contrôles-qualité)
- [Sécurité](#sécurité)
- [Index de la documentation](#index-de-la-documentation)

---

## Auteurs

| Contributeur | Rôle |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | Architecture, cœur de la plateforme |
| **Gemini AI** | Génération de code, documentation |
| **Claude AI** | Analyse de code, audit de sécurité |

---

## Capacités principales

### 1. Monolithe modulaire haute performance

- **Modules Go compilés** — le cœur n'embarque que `sso_clients`. Les
  distributions produit enregistrent leurs modules via le contrat public
  `pkg/nexus` dans le binaire final, où ils sont appelés en processus.
- **Magasin d'applications par locataire** — droits applicatifs, menus et RBAC
  sont pilotés depuis PostgreSQL (`app_installations`).
- **Résolveur de dépendances** — résolution récursive sur un graphe orienté
  acyclique, avec détection de cycles et vérification des contraintes semver.
- **Synchronisation du catalogue** — la production récupère un catalogue signé
  via `APP_CATALOG_URL`; le mode développement/hors ligne utilise
  `catalog/apps.json`, puis synchronise les métadonnées dans `platform.apps`.

### 2. Résilience cloud-native et réplicas multiples

| Module | Rôle |
| --- | --- |
| `internal/kernel/resilience/loadshedder.go` | Délestage avec `503` + `Retry-After` sous charge |
| `internal/kernel/cache/bus.go` | Invalidation Redis entre réplicas, avec repli local |
| `internal/kernel/memo/memo.go` | Cache local à TTL court, invalidé par préfixe, pour les décisions d'autorisation |
| `internal/kernel/async/async.go` | Goroutines nommées avec récupération de panic et journal de pile |

### 3. Infrastructure numérique nationale

- **XYP — échange d'informations de l'État** (`internal/workspace/identity/gerege/xyp.go`) :
  registre civil des citoyens (`WS100101`) et vérification des personnes
  morales (`WS100201`).
- **E-ID national et DAN** ([`developer.gerege.mn`](https://developer.gerege.mn),
  [`eidmongolia.mn`](https://eidmongolia.mn)) — signature numérique PKI, OTP
  mobile, SSO bancaire et vérification faciale biométrique.
- **Fournisseur OAuth2 / OIDC intégré**
  (`/.well-known/openid-configuration`) délivrant des jetons
  client-credentials à des systèmes tiers.
- **Vérification d'e-mail** (`internal/workspace/emailverify`) — un flux partagé pour
  prouver une adresse, appelé en interne par chaque module applicatif. L'e-mail
  est envoyé par le service hébergé (`enigma.mn`) : la plateforme ne détient
  aucune information d'authentification de messagerie et ne possède pas
  d'adresse d'expéditeur. La vérification est enregistrée au retour de la
  personne, et ce retour ne fonctionne qu'une fois. Visible dans Paramètres →
  Vérification d'e-mail.

> **Remarque.** Le mode simulé (mock) pour E-ID, DAN et XYP est une commodité de
> développement uniquement. Avec `ENVIRONMENT=production` il est désactivé
> automatiquement : un numéro d'enregistrement fabriqué ne peut jamais
> authentifier.

### 4. Copilote IA et analytique

- **Assistant IA** (`internal/workspace/ai/copilot.go`) — conversation classée par
  intention, branchée sur les données réelles du locataire.
- **Prévision du stock** (`internal/workspace/ai/handlers.go`) — délègue à la
  capacité `stock_forecast` d'une distribution activée et renvoie `404` si
  aucun module ne la fournit.

---

## Applications métier

Ce dépôt de base ne fournit qu'une seule application dans `catalog/apps.json`.
Les distributions de produit enregistrent leurs propres modules et migrations
via `pkg/nexus` ; leurs applications ne sont pas des fonctions incluses ici.

| # | Application | ID | Route | Description |
| --- | --- | --- | --- | --- |
| 1 | Clients SSO | `io.gerege.nexus.sso_clients` | `/sso-clients` | Clients OAuth2 des systèmes qui connectent des personnes via cette plateforme |

Les routes ne s'ouvrent qu'une fois l'application installée et activée pour le
locataire ; sinon le contrôle renvoie `403 Forbidden`.

---

## Structure du dépôt

```
backend/
  cmd/api/            Serveur d'API HTTP (+ jeu de données de démonstration)
  cmd/migrate/        Exécuteur de migrations Goose
  db/migrations/      Migrations SQL
  internal/
    kernel/           Primitives techniques communes
    tenant/           Travail pour une organisation
    platform/         Opérations sur tout le déploiement
    apps/             Modules inclus par cette distribution
  pkg/
    nexus/            SDK public et contrats des modules externes
    platform/         Racine de composition des deux plans
frontend/             Client web Next.js 16 (App Router)
catalog/              Catalogue et manifestes du magasin d'applications
deploy/               Dockerfile de production, configuration Nginx
docs/                 Documentation et traductions
```

---

## Démarrage

### Prérequis

- Go 1.26+
- Node.js 20+
- PostgreSQL 16+ (ou Docker Compose)

### 1. Docker Compose

```bash
docker compose up -d
```

Les migrations s'exécutent dans un service `migrate` dédié, à usage unique,
avant le démarrage de l'API.

### 2. Manuellement

**Backend :**

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

**Frontend :**

```bash
cd frontend
npm ci
npm run dev
```

Ouvrez [http://localhost:3000](http://localhost:3000).

### Identifiants de démonstration

| Champ | Valeur |
| --- | --- |
| E-mail | `admin@example.com` |
| Mot de passe | `Password123!` |
| Locataire | `Demo Corporation` (`slug: demo`) |

Le compte de démonstration n'est créé qu'en dehors de la production. En
production il n'est créé que si `SEED_DEMO_DATA=true` est défini explicitement.

---

## Déploiement automatisé

Chaque poussée sur `main` déclenche [`deploy.yml`](../.github/workflows/deploy.yml) :

1. Construire et publier les images backend et frontend sur GHCR (`:latest` et `:<sha>`).
2. Copier `docker-compose.prod.yml` sur le serveur.
3. Écrire le `.env` du serveur depuis les secrets GitHub et récupérer les images.
4. Exécuter les migrations jusqu'au bout, puis basculer l'API et le frontend.
5. Sonder `/health` et `/ready`, afficher les journaux des conteneurs et faire
   échouer l'exécution si le déploiement n'est pas sain.

Déploiement manuel : Actions → *Deploy to Production* → **Run workflow**, en
épinglant éventuellement une étiquette d'image.

Secrets requis dans le dépôt :

| Secret | Requis | Description |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Oui | Clé privée de l'utilisateur de déploiement. Sans elle, le déploiement est ignoré |
| `POSTGRES_PASSWORD` | Oui | Mot de passe de la base de données sur le serveur |
| `SSO_DEFAULT_CLIENT_SECRET` | Oui | Obligatoire pour le client OAuth2 intégré en production |
| `DEPLOY_HOST` / `DEPLOY_USER` / `DEPLOY_PORT` | Non | Par défaut `nexus.gerege.mn` / `deploy` / `22` |
| `PUBLIC_ORIGIN` | Non | Par défaut `https://nexus.gerege.mn` |

> Le domaine de production est `nexus.gerege.mn`, qui a remplacé
> `openerp.gerege.mn` lors du changement de nom vers Gerege Nexus.
> `PUBLIC_ORIGIN` définit en un seul endroit le CORS, l'émetteur OIDC et le
> callback eID : le déplacer entraîne donc le DNS, le certificat TLS et tout
> client ayant épinglé l'émetteur.

Le serveur n'a besoin que de Docker — ni code source, ni chaîne d'outils
Go/Node. Voir [`deploy/.env.prod.example`](../deploy/.env.prod.example) pour les
valeurs.

---

## Configuration

Voir [`.env.example`](../.env.example) pour la liste complète.

| Variable | Défaut | Description |
| --- | --- | --- |
| `DATABASE_URL` | localhost | Chaîne de connexion PostgreSQL |
| `PORT` | `8080` | Port d'écoute de l'API |
| `ENVIRONMENT` | `development` | `production` active les valeurs durcies |
| `APP_CATALOG_PATH` | `catalog/apps.json` | Chemin du catalogue du magasin d'applications |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | Liste d'origines autorisées (CORS) |
| `TRUST_PROXY_HEADERS` | `false` | Faut-il faire confiance à `X-Forwarded-For` |
| `SEED_DEMO_DATA` | activé hors production | Créer le compte de démonstration |
| `SSO_DEFAULT_CLIENT_SECRET` | — | Requis en production |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | activé hors production | Simuler les intégrations nationales |

---

## Aperçu de l'API

| Méthode | Chemin | Description |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | Sondes de vivacité et de disponibilité |
| `GET` | `/metrics` | Métriques Prometheus |
| `POST` | `/api/v1/auth/login` | Connexion par e-mail et mot de passe |
| `POST` | `/api/v1/auth/eid/login` | Connexion par E-ID national |
| `POST` | `/api/v1/auth/dan/login` | Connexion via la passerelle DAN |
| `POST` | `/api/v1/auth/logout` | Révoquer la session |
| `GET` | `/api/v1/menus` | Menus des applications activées pour le locataire |
| `GET` | `/api/v1/store/apps` | Liste du magasin d'applications |
| `POST` | `/api/v1/store/apps/{slug}/install` | Installer une application (admin) |
| `POST` | `/api/v1/verify/send` | Demander un lien de vérification au service hébergé |
| `GET` | `/api/v1/verify/landed` | Recevoir la personne qui a confirmé — valable une seule fois |
| `GET` | `/api/platform/v1/email-verifications` | Registre des vérifications et état du service (console) |
| `POST` | `/oauth2/token` | Jeton OAuth2 client credentials |

Les jetons de session circulent soit dans le cookie HttpOnly, soit via
`Authorization: Bearer <token>`.

---

## Tests et contrôles qualité

```bash
# Tests unitaires backend avec le détecteur de courses
cd backend && go test -race ./...

# Analyse statique
cd backend && go vet ./... && golangci-lint run

# Analyse des vulnérabilités
cd backend && govulncheck ./...

# Build du frontend
cd frontend && npm run build
```

La CI exécute le lint, les tests, le build du frontend, la construction de
l'image Docker, govulncheck et gosec à chaque poussée et chaque pull request.

---

## Sécurité

- Les jetons de session sont des valeurs aléatoires de 256 bits ; seul leur
  condensé SHA-256 est stocké.
- Les mots de passe sont hachés avec bcrypt et les tentatives de connexion sont
  limitées par IP.
- Installer, activer ou désactiver des applications et enregistrer des
  intégrations exige les droits d'administrateur du locataire.
- L'authentification des clients OAuth2 utilise une comparaison à temps
  constant.

Signalez les vulnérabilités comme décrit dans [`SECURITY.md`](../SECURITY.md).

---

## Index de la documentation

| Document | Description |
| --- | --- |
| [Centre de documentation](README.md) | Index de tous les documents et traductions |
| [Spécification d'architecture](ARCHITECTURE_SPECIFICATION.md) | Couches de la plateforme et décisions de conception |
| [Guide de création de module](MODULE_AUTHORING_GUIDE.md) | Comment construire un nouveau module applicatif |
| [Contribuer](../CONTRIBUTING.md) | Processus de contribution |
| [Politique de sécurité](../SECURITY.md) | Signalement des vulnérabilités |
| [Code de conduite](../CODE_OF_CONDUCT.md) | Règles de la communauté |
| [Journal des modifications](../CHANGELOG.md) | Historique des versions |

---

## Remerciements et inspirations

1. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
   de **[@snykk](https://github.com/snykk)** — fondations de l'API REST Go.
2. **[Odoo](https://github.com/odoo/odoo)** — magasin d'applications modulaire
   et modèle de dépendances.
3. **[go-zero](https://github.com/zeromicro/go-zero)** — moteur de résilience
   cloud-native.

---

## Licence

Copyright (c) 2026 **Gerege Systems Development Team, Gerege Nomadica Foundation**. Distribué sous licence Apache 2.0 — voir
[`LICENSE`](../LICENSE).

Icônes de drapeaux par [Flaticon](https://www.flaticon.com/)
([attribution](assets/icons/ATTRIBUTION.md)).
