# Gerege Nexus

**Plataforma Integrada de Operaciones Digitales**

**Gerege Nexus** es una plataforma modular de código abierto que conecta
servicios, operaciones, sistemas y datos de organizaciones públicas y privadas.
Está diseñada con el **mongol como idioma principal** y se integra directamente
con la infraestructura digital nacional de Mongolia (DAN, E-ID, XYP / ХУР).

*Nexus* es el punto de conexión: donde confluyen organizaciones, servicios,
flujos de trabajo, sistemas, usuarios y datos. La plataforma en sí no está
ligada a ningún sector — son los módulos que se ejecutan sobre ella los que
definen cada despliegue.

Los módulos se compilan en un único binario de Go, mientras que una tienda de
aplicaciones respaldada por PostgreSQL decide qué aplicaciones están activas por
inquilino — separación modular sin los saltos de red ni el coste operativo de
los microservicios.

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_AR.md"><img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> العربية</a>
  &nbsp;·&nbsp;
  <a href="README_ZH.md"><img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> 中文</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
  &nbsp;·&nbsp;
  <a href="README_FR.md"><img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> Français</a>
  &nbsp;·&nbsp;
  <a href="README_RU.md"><img src="assets/icons/flag-ru.png" width="18" height="18" alt=""> Русский</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-es.png" width="18" height="18" alt=""> <b>Español</b>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16.3-black.svg)](https://nextjs.org)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../CONTRIBUTING.md)

---

## Contenido

- [Autores](#autores)
- [Capacidades principales](#capacidades-principales)
- [Aplicaciones de negocio](#aplicaciones-de-negocio)
- [Estructura del repositorio](#estructura-del-repositorio)
- [Primeros pasos](#primeros-pasos)
- [Configuración](#configuración)
- [Resumen de la API](#resumen-de-la-api)
- [Pruebas y controles de calidad](#pruebas-y-controles-de-calidad)
- [Seguridad](#seguridad)
- [Índice de documentación](#índice-de-documentación)

---

## Autores

| Colaborador | Función |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | Arquitectura, núcleo de la plataforma |
| **Gemini AI** | Generación de código, documentación |
| **Claude AI** | Análisis de código, auditoría de seguridad |

---

## Capacidades principales

### 1. Monolito modular de alto rendimiento

- **Módulos Go compilados** — el núcleo solo incluye `sso_clients`. Las
  distribuciones de producto registran sus módulos mediante el contrato
  público `pkg/nexus` en el binario final, donde se invocan en proceso.
- **Tienda de aplicaciones por inquilino** — los permisos de aplicación, los
  menús y el RBAC se gobiernan desde PostgreSQL (`app_installations`).
- **Resolutor de dependencias** — resolución recursiva sobre un grafo dirigido
  acíclico, con detección de ciclos y verificación de restricciones semver.
- **Sincronización del catálogo** — producción obtiene un catálogo firmado de
  `APP_CATALOG_URL`; desarrollo/offline usa `catalog/apps.json` como respaldo y
  sincroniza los metadatos en `platform.apps`.

### 2. Resiliencia cloud-native y múltiples réplicas

| Módulo | Propósito |
| --- | --- |
| `internal/kernel/resilience/loadshedder.go` | Descarta carga con `503` + `Retry-After` bajo presión |
| `internal/kernel/cache/bus.go` | Invalida caché entre réplicas mediante Redis, con respaldo local |
| `internal/kernel/memo/memo.go` | Caché local de TTL corto, invalidada por prefijo, para decisiones de autorización |
| `internal/kernel/async/async.go` | Goroutines con nombre, recuperación de panic y registro de pila |

### 3. Infraestructura digital nacional

- **XYP — Intercambio de Información del Estado** (`internal/workspace/identity/gerege/xyp.go`):
  registro civil de ciudadanos (`WS100101`) y verificación de personas
  jurídicas (`WS100201`).
- **E-ID nacional y DAN** ([`developer.gerege.mn`](https://developer.gerege.mn),
  [`eidmongolia.mn`](https://eidmongolia.mn)) — firma digital PKI, OTP móvil,
  SSO bancario y verificación facial biométrica.
- **Proveedor OAuth2 / OIDC integrado**
  (`/.well-known/openid-configuration`) que emite tokens de tipo
  client-credentials a sistemas de terceros.
- **Verificación de correo electrónico** (`internal/workspace/emailverify`) — un único
  flujo para demostrar una dirección, que cada módulo de aplicación llama en
  proceso. El correo lo envía el servicio alojado (`enigma.mn`), de modo que la
  plataforma no guarda credenciales de buzón ni posee dirección de remitente. La
  verificación se registra cuando la persona vuelve, y ese retorno sirve una
  sola vez. Visible en Ajustes → Verificación de correo.

> **Nota.** El modo simulado (mock) de E-ID, DAN y XYP es únicamente una
> comodidad de desarrollo. Con `ENVIRONMENT=production` se desactiva
> automáticamente, de modo que un número de registro inventado nunca puede
> autenticarse.

### 4. Copiloto de IA y analítica

- **Asistente de IA** (`internal/workspace/ai/copilot.go`) — conversación clasificada por
  intención y conectada a los datos reales del inquilino.
- **Previsión de inventario** (`internal/workspace/ai/handlers.go`) — delega en la
  capacidad `stock_forecast` de una distribución habilitada y devuelve `404`
  cuando ningún módulo la ofrece.

---

## Aplicaciones de negocio

Este repositorio base solo incluye una aplicación en `catalog/apps.json`.
Las distribuciones de producto registran sus propios módulos y migraciones
mediante `pkg/nexus`; sus aplicaciones no son funciones incluidas aquí.

| # | Aplicación | ID | Ruta | Descripción |
| --- | --- | --- | --- | --- |
| 1 | Clientes SSO | `io.gerege.nexus.sso_clients` | `/sso-clients` | Clientes OAuth2 de los sistemas que inician sesión de personas a través de esta plataforma |

Las rutas solo se abren una vez que la aplicación está instalada y habilitada
para el inquilino; de lo contrario el control devuelve `403 Forbidden`.

---

## Estructura del repositorio

```
backend/
  cmd/api/            Servidor de la API HTTP (+ datos de demostración)
  cmd/migrate/        Ejecutor de migraciones Goose
  db/migrations/      Migraciones SQL
  internal/
    kernel/           Primitivas técnicas comunes
    tenant/           Trabajo para una organización
    platform/         Operación de todo el deployment
    apps/             Módulos incluidos por esta distribución
  pkg/
    nexus/            SDK público y contratos de módulos externos
    platform/         Raíz de composición de ambos planos
frontend/             Cliente web Next.js 16 (App Router)
catalog/              Catálogo y manifiestos de la tienda de aplicaciones
deploy/               Dockerfile de producción, configuración de Nginx
docs/                 Documentación y traducciones
```

---

## Primeros pasos

### Requisitos previos

- Go 1.26+
- Node.js 20+
- PostgreSQL 16+ (o Docker Compose)

### 1. Docker Compose

```bash
docker compose up -d
```

Las migraciones se ejecutan en un servicio `migrate` dedicado y de un solo uso
antes de que arranque la API.

### 2. Manualmente

**Backend:**

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

**Frontend:**

```bash
cd frontend
npm ci
npm run dev
```

Abra [http://localhost:3000](http://localhost:3000).

### Credenciales de demostración

| Campo | Valor |
| --- | --- |
| Correo | `admin@example.com` |
| Contraseña | `Password123!` |
| Inquilino | `Demo Corporation` (`slug: demo`) |

La cuenta de demostración solo se crea fuera de producción. En producción se
crea únicamente cuando se establece `SEED_DEMO_DATA=true` de forma explícita.

---

## Despliegue automatizado

Cada push a `main` ejecuta [`deploy.yml`](../.github/workflows/deploy.yml):

1. Construir y publicar las imágenes de backend y frontend en GHCR (`:latest` y `:<sha>`).
2. Copiar `docker-compose.prod.yml` al servidor.
3. Escribir el `.env` del servidor desde los secretos de GitHub y descargar las imágenes.
4. Ejecutar las migraciones hasta el final y luego conmutar la API y el frontend.
5. Sondear `/health` y `/ready`, mostrar los registros de los contenedores y
   marcar la ejecución como fallida si el despliegue no está sano.

Despliegue manual: Actions → *Deploy to Production* → **Run workflow**, fijando
opcionalmente una etiqueta de imagen.

Secretos requeridos en el repositorio:

| Secreto | Requerido | Descripción |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Sí | Clave privada del usuario de despliegue. Sin ella se omite el despliegue |
| `POSTGRES_PASSWORD` | Sí | Contraseña de la base de datos en el servidor |
| `SSO_DEFAULT_CLIENT_SECRET` | Sí | Obligatorio para el cliente OAuth2 integrado en producción |
| `DEPLOY_HOST` / `DEPLOY_USER` / `DEPLOY_PORT` | No | Por defecto `nexus.gerege.mn` / `deploy` / `22` |
| `PUBLIC_ORIGIN` | No | Por defecto `https://nexus.gerege.mn` |

> El dominio de producción es `nexus.gerege.mn`, que sustituyó a
> `openerp.gerege.mn` en el cambio de nombre a Gerege Nexus. `PUBLIC_ORIGIN`
> define en un mismo lugar el CORS, el emisor OIDC y el callback de eID, de modo
> que moverlo arrastra consigo el DNS, el certificado TLS y todo cliente que
> haya fijado el emisor.

El servidor solo necesita Docker — sin código fuente ni cadena de herramientas
Go/Node. Consulte [`deploy/.env.prod.example`](../deploy/.env.prod.example) para
los valores.

---

## Configuración

Consulte [`.env.example`](../.env.example) para la lista completa.

| Variable | Predeterminado | Descripción |
| --- | --- | --- |
| `DATABASE_URL` | localhost | Cadena de conexión a PostgreSQL |
| `PORT` | `8080` | Puerto de escucha de la API |
| `ENVIRONMENT` | `development` | `production` activa valores endurecidos |
| `APP_CATALOG_PATH` | `catalog/apps.json` | Ruta del catálogo de la tienda de aplicaciones |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | Lista de orígenes permitidos (CORS) |
| `TRUST_PROXY_HEADERS` | `false` | Si se confía en `X-Forwarded-For` |
| `SEED_DEMO_DATA` | activo fuera de producción | Crear la cuenta de demostración |
| `SSO_DEFAULT_CLIENT_SECRET` | — | Requerido en producción |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | activo fuera de producción | Simular las integraciones nacionales |

---

## Resumen de la API

| Método | Ruta | Descripción |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | Sondas de vitalidad y disponibilidad |
| `GET` | `/metrics` | Métricas de Prometheus |
| `POST` | `/api/v1/auth/login` | Inicio de sesión con correo y contraseña |
| `POST` | `/api/v1/auth/eid/login` | Inicio de sesión con E-ID nacional |
| `POST` | `/api/v1/auth/dan/login` | Inicio de sesión mediante la pasarela DAN |
| `POST` | `/api/v1/auth/logout` | Revocar la sesión |
| `GET` | `/api/v1/menus` | Menús de las aplicaciones habilitadas del inquilino |
| `GET` | `/api/v1/store/apps` | Listado de la tienda de aplicaciones |
| `POST` | `/api/v1/store/apps/{slug}/install` | Instalar una aplicación (admin) |
| `POST` | `/api/v1/verify/send` | Solicitar un enlace de verificación al servicio alojado |
| `GET` | `/api/v1/verify/landed` | Recibir a quien ha confirmado — sirve una sola vez |
| `GET` | `/api/platform/v1/email-verifications` | Registro de verificaciones y estado del servicio (consola) |
| `POST` | `/oauth2/token` | Token OAuth2 de client credentials |

Los tokens de sesión viajan en la cookie HttpOnly o como
`Authorization: Bearer <token>`.

---

## Pruebas y controles de calidad

```bash
# Pruebas unitarias del backend con el detector de carreras
cd backend && go test -race ./...

# Análisis estático
cd backend && go vet ./... && golangci-lint run

# Análisis de vulnerabilidades
cd backend && govulncheck ./...

# Compilación del frontend
cd frontend && npm run build
```

La CI ejecuta lint, pruebas, la compilación del frontend, la construcción de la
imagen Docker, govulncheck y gosec en cada push y cada pull request.

---

## Seguridad

- Los tokens de sesión son valores aleatorios de 256 bits; solo se almacena su
  resumen SHA-256.
- Las contraseñas se cifran con bcrypt y los intentos de inicio de sesión están
  limitados por IP.
- Instalar, habilitar o deshabilitar aplicaciones y registrar integraciones
  requiere derechos de administrador del inquilino.
- La autenticación de clientes OAuth2 usa comparación en tiempo constante.

Informe de vulnerabilidades según lo descrito en [`SECURITY.md`](../SECURITY.md).

---

## Índice de documentación

| Documento | Descripción |
| --- | --- |
| [Centro de documentación](README.md) | Índice de todos los documentos y traducciones |
| [Especificación de arquitectura](ARCHITECTURE_SPECIFICATION.md) | Capas de la plataforma y decisiones de diseño |
| [Guía de creación de módulos](MODULE_AUTHORING_GUIDE.md) | Cómo construir un nuevo módulo de aplicación |
| [Contribuir](../CONTRIBUTING.md) | Flujo de contribución |
| [Política de seguridad](../SECURITY.md) | Notificación de vulnerabilidades |
| [Código de conducta](../CODE_OF_CONDUCT.md) | Normas de la comunidad |
| [Registro de cambios](../CHANGELOG.md) | Historial de versiones |

---

## Créditos e inspiración

1. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
   de **[@snykk](https://github.com/snykk)** — fundamentos de la API REST en Go.
2. **[Odoo](https://github.com/odoo/odoo)** — tienda de aplicaciones modular y
   modelo de dependencias.
3. **[go-zero](https://github.com/zeromicro/go-zero)** — motor de resiliencia
   cloud-native.

---

## Licencia

Copyright (c) 2026 **Gerege Systems Development Team, Gerege Nomadica Foundation**. Distribuido bajo la Licencia Apache 2.0 — véase
[`LICENSE`](../LICENSE).

Iconos de banderas por [Flaticon](https://www.flaticon.com/)
([atribución](assets/icons/ATTRIBUTION.md)).
