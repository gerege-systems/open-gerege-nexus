# Gerege Nexus

**Единая платформа цифровых операций**

**Gerege Nexus** — это открытая модульная платформа, объединяющая услуги,
операции, системы и данные государственных и частных организаций. Основной язык
платформы — **монгольский**, и она напрямую интегрирована с национальной
цифровой инфраструктурой Монголии (DAN, E-ID, XYP / ХУР).

*Nexus* — это точка соединения: место, где сходятся организации, услуги,
рабочие процессы, системы, пользователи и данные. Сама платформа не привязана к
одной отрасли — специфику задаёт набор работающих на ней модулей.

Модули компилируются в единый бинарный файл Go, а магазин приложений на
PostgreSQL определяет, какие приложения активны для каждого арендатора, —
разделение модулей без сетевых вызовов и эксплуатационной сложности
микросервисов.

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
  <img src="assets/icons/flag-ru.png" width="18" height="18" alt=""> <b>Русский</b>
  &nbsp;·&nbsp;
  <a href="README_ES.md"><img src="assets/icons/flag-es.png" width="18" height="18" alt=""> Español</a>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16.3-black.svg)](https://nextjs.org)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../CONTRIBUTING.md)

---

## Содержание

- [Авторы](#авторы)
- [Ключевые возможности](#ключевые-возможности)
- [Бизнес-приложения](#бизнес-приложения)
- [Структура репозитория](#структура-репозитория)
- [Быстрый старт](#быстрый-старт)
- [Конфигурация](#конфигурация)
- [Обзор API](#обзор-api)
- [Тесты и контроль качества](#тесты-и-контроль-качества)
- [Безопасность](#безопасность)
- [Указатель документации](#указатель-документации)

---

## Авторы

| Участник | Роль |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | Архитектура, ядро платформы |
| **Gemini AI** | Генерация кода, документация |
| **Claude AI** | Анализ кода, аудит безопасности |

---

## Ключевые возможности

### 1. Высокопроизводительный модульный монолит

- **Go-модули времени компиляции** — ядро содержит только `sso_clients`.
  Продуктовые дистрибутивы регистрируют модули через публичный контракт
  `pkg/nexus` в итоговом бинарном файле и вызывают их внутри процесса.
- **Магазин приложений на уровне арендатора** — права на приложения, меню и RBAC
  управляются через PostgreSQL (`app_installations`).
- **Разрешение зависимостей** — рекурсивный обход направленного ациклического
  графа с обнаружением циклов и проверкой semver-ограничений.
- **Синхронизация каталога** — production загружает подписанный каталог из
  `APP_CATALOG_URL`; development/offline использует `catalog/apps.json` как
  fallback и синхронизирует метаданные в `platform.apps`.

### 2. Cloud-native отказоустойчивость и несколько реплик

| Модуль | Назначение |
| --- | --- |
| `internal/kernel/resilience/loadshedder.go` | Отбрасывает нагрузку с `503` и `Retry-After` |
| `internal/kernel/cache/bus.go` | Инвалидация между репликами через Redis с локальным fallback |
| `internal/kernel/memo/memo.go` | Локальный TTL-кэш решений авторизации с инвалидацией по префиксу |
| `internal/kernel/async/async.go` | Именованные goroutine с восстановлением после panic и логом стека |

### 3. Интеграция с национальной инфраструктурой

- **XYP — государственная система обмена данными**
  (`internal/workspace/identity/gerege/xyp.go`): регистрация граждан (`WS100101`) и проверка
  юридических лиц (`WS100201`).
- **Национальные E-ID и DAN** ([`developer.gerege.mn`](https://developer.gerege.mn),
  [`eidmongolia.mn`](https://eidmongolia.mn)) — цифровая подпись PKI, мобильный
  OTP, банковский SSO и биометрия по лицу.
- **Встроенный провайдер OAuth2 / OIDC**
  (`/.well-known/openid-configuration`), выдающий токены по схеме
  client credentials.
- **Подтверждение адреса электронной почты** (`internal/workspace/emailverify`) — единый
  процесс подтверждения, который все модули приложений вызывают внутри
  процесса. Письмо отправляет хостинговая служба (`enigma.mn`), поэтому
  платформа не хранит почтовых учётных данных и не владеет адресом отправителя.
  Подтверждение записывается, когда человек возвращается, и этот возврат
  срабатывает ровно один раз. Виден в разделе «Настройки → Подтверждение
  адреса».

> **Важно.** Mock-режим для E-ID, DAN и XYP предназначен только для разработки.
> При `ENVIRONMENT=production` он отключается автоматически, поэтому
> сфабрикованный регистрационный номер не пройдёт аутентификацию.

### 4. AI-помощник и аналитика

- **AI-ассистент** (`internal/workspace/ai/copilot.go`) — диалог с классификацией
  намерений, подключённый к актуальным данным арендатора.
- **Прогноз запасов** (`internal/workspace/ai/handlers.go`) — делегируется
  capability `stock_forecast` включённого дистрибутива и возвращает `404`,
  если ни один модуль её не предоставляет.

---

## Бизнес-приложения

В `catalog/apps.json` этого базового репозитория поставляется только одно
приложение. Продуктовые дистрибутивы регистрируют свои модули и миграции через
`pkg/nexus`; их приложения не являются функциями данного репозитория.

| # | Приложение | ID | Маршрут | Описание |
| --- | --- | --- | --- | --- |
| 1 | SSO-клиенты | `io.gerege.nexus.sso_clients` | `/sso-clients` | OAuth2-клиенты систем, которые выполняют вход пользователей через эту платформу |

Маршруты открываются только после установки и включения приложения для
арендатора, иначе шлюз возвращает `403 Forbidden`.

---

## Структура репозитория

```
backend/
  cmd/api/            HTTP API-сервер (+ demo seeder)
  cmd/migrate/        Запуск миграций Goose
  db/migrations/      SQL-миграции
  internal/
    kernel/           Общие технические примитивы
    tenant/           Работа в рамках одной организации
    platform/         Управление всем deployment
    apps/             Модули этого дистрибутива
  pkg/
    nexus/            Публичный SDK и контракты внешних модулей
    platform/         Composition root двух плоскостей
frontend/             Веб-клиент Next.js 16 (App Router)
catalog/              Каталог магазина приложений и манифесты
deploy/               Production Dockerfile, конфигурация Nginx
docs/                 Документация и переводы
```

---

## Быстрый старт

### Требования

- Go 1.26+
- Node.js 20+
- PostgreSQL 16+ (или Docker Compose)

### 1. Docker Compose

```bash
docker compose up -d
```

Миграции выполняет отдельный одноразовый сервис `migrate`, и только после этого
запускается API.

### 2. Вручную

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

Откройте [http://localhost:3000](http://localhost:3000).

### Демонстрационный доступ

| Поле | Значение |
| --- | --- |
| Email | `admin@example.com` |
| Пароль | `Password123!` |
| Арендатор | `Demo Corporation` (`slug: demo`) |

Демо-учётная запись создаётся только вне production. В production она появится
лишь при явно заданном `SEED_DEMO_DATA=true`.

---

## Конфигурация

Полный список — в [`.env.example`](../.env.example).

| Переменная | По умолчанию | Описание |
| --- | --- | --- |
| `DATABASE_URL` | localhost | Строка подключения к PostgreSQL |
| `PORT` | `8080` | Порт API |
| `ENVIRONMENT` | `development` | `production` включает усиленные настройки |
| `APP_CATALOG_PATH` | `catalog/apps.json` | Путь к каталогу приложений |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | Список разрешённых источников CORS |
| `TRUST_PROXY_HEADERS` | `false` | Доверять ли `X-Forwarded-For` |
| `SEED_DEMO_DATA` | включено вне production | Создание демо-учётной записи |
| `SSO_DEFAULT_CLIENT_SECRET` | — | Обязательно в production |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | включено вне production | Mock-режим государственных систем |

---

## Обзор API

| Метод | Путь | Описание |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | Проверки живости и готовности |
| `GET` | `/metrics` | Метрики Prometheus |
| `POST` | `/api/v1/auth/login` | Вход по email и паролю |
| `POST` | `/api/v1/auth/eid/login` | Вход через национальный E-ID |
| `POST` | `/api/v1/auth/dan/login` | Вход через шлюз DAN |
| `POST` | `/api/v1/auth/logout` | Отзыв сессии |
| `GET` | `/api/v1/menus` | Меню включённых приложений арендатора |
| `GET` | `/api/v1/store/apps` | Список магазина приложений |
| `POST` | `/api/v1/store/apps/{slug}/install` | Установка приложения (админ) |
| `POST` | `/api/v1/verify/send` | Запросить ссылку подтверждения у хостинговой службы |
| `GET` | `/api/v1/verify/landed` | Принять того, кто подтвердил адрес — срабатывает один раз |
| `GET` | `/api/v1/admin/email-verification/overview` | История подтверждений и состояние службы (админ) |
| `POST` | `/oauth2/token` | Токен OAuth2 client credentials |

Токен сессии передаётся в HttpOnly cookie либо в заголовке
`Authorization: Bearer <token>`.

---

## Тесты и контроль качества

```bash
# Модульные тесты backend с детектором гонок
cd backend && go test -race ./...

# Статический анализ
cd backend && go vet ./... && golangci-lint run

# Проверка уязвимостей
cd backend && govulncheck ./...

# Сборка frontend
cd frontend && npm run build
```

CI выполняет lint, тесты, сборку frontend, сборку Docker-образа, govulncheck и
gosec при каждом push и pull request.

---

## Безопасность

- Токены сессий — 256-битные случайные значения; в базе хранится только их
  SHA-256-дайджест.
- Пароли хешируются bcrypt, попытки входа ограничиваются по IP.
- Установка, включение и отключение приложений, а также регистрация интеграций
  требуют прав администратора арендатора.
- Аутентификация OAuth2-клиента использует сравнение за постоянное время.

Порядок сообщения об уязвимостях описан в [`SECURITY.md`](../SECURITY.md).

---

## Указатель документации

| Документ | Описание |
| --- | --- |
| [Центр документации](README.md) | Указатель всех документов и переводов |
| [Спецификация архитектуры](ARCHITECTURE_SPECIFICATION.md) | Слои платформы и проектные решения |
| [Руководство по разработке модулей](MODULE_AUTHORING_GUIDE.md) | Как создать новый модуль приложения |
| [Руководство для контрибьюторов](../CONTRIBUTING.md) | Процесс внесения вклада |
| [Политика безопасности](../SECURITY.md) | Сообщение об уязвимостях |
| [Кодекс поведения](../CODE_OF_CONDUCT.md) | Нормы сообщества |
| [История изменений](../CHANGELOG.md) | История релизов |

---

## Благодарности и источники вдохновения

1. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
   от **[@snykk](https://github.com/snykk)** — основа Go REST API.
2. **[Odoo](https://github.com/odoo/odoo)** — модульный магазин приложений и
   модель зависимостей.
3. **[go-zero](https://github.com/zeromicro/go-zero)** — cloud-native механизмы
   отказоустойчивости.

---

## Лицензия

Copyright (c) 2026 **Gerege Systems Development Team, Gerege Nomadica Foundation**. Распространяется по лицензии Apache 2.0 — см.
[`LICENSE`](../LICENSE).

Иконки флагов — [Flaticon](https://www.flaticon.com/)
([атрибуция](assets/icons/ATTRIBUTION.md)).
