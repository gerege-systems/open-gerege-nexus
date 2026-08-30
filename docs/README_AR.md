<div dir="rtl">

# Gerege Nexus

**منصة متكاملة للعمليات الرقمية**

**Gerege Nexus** منصة معيارية مفتوحة المصدر تربط الخدمات والعمليات والأنظمة
والبيانات عبر المؤسسات العامة والخاصة. تضع اللغة **المنغولية في المقام الأول**،
وتتكامل مباشرة مع البنية التحتية الرقمية الوطنية في منغوليا (DAN و E-ID و
XYP / ХУР).

كلمة *Nexus* تعني نقطة الاتصال: حيث تلتقي المؤسسات والخدمات وسير العمل والأنظمة
والمستخدمون والبيانات. المنصة نفسها ليست مرتبطة بقطاع واحد — الوحدات التي تعمل
فوقها هي ما يحدد طبيعة كل عملية نشر.

تُجمَّع الوحدات في ملف تنفيذي واحد بلغة Go، بينما يقرر متجر تطبيقات مدعوم بـ
PostgreSQL أي التطبيقات مفعَّلة لكل مستأجر — فصل معياري دون قفزات الشبكة أو
الكلفة التشغيلية للخدمات المصغَّرة.

</div>

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> <b>العربية</b>
  &nbsp;·&nbsp;
  <a href="README_ZH.md"><img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> 中文</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
  &nbsp;·&nbsp;
  <a href="README_FR.md"><img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> Français</a>
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

<div dir="rtl">

## المحتويات

- [المؤلفون](#المؤلفون)
- [القدرات الأساسية](#القدرات-الأساسية)
- [تطبيقات الأعمال](#تطبيقات-الأعمال)
- [بنية المستودع](#بنية-المستودع)
- [البدء](#البدء)
- [الإعدادات](#الإعدادات)
- [نظرة عامة على الواجهة البرمجية](#نظرة-عامة-على-الواجهة-البرمجية)
- [الاختبارات وضوابط الجودة](#الاختبارات-وضوابط-الجودة)
- [الأمان](#الأمان)
- [فهرس التوثيق](#فهرس-التوثيق)

---

## المؤلفون

</div>

| المساهم | الدور |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | البنية المعمارية، نواة المنصة |
| **Gemini AI** | توليد الشيفرة، التوثيق |
| **Claude AI** | تحليل الشيفرة، تدقيق الأمان |

---

<div dir="rtl">

## القدرات الأساسية

### ١. نواة معيارية أحادية عالية الأداء

- **وحدات Go مُجمَّعة وقت البناء** — يحمل النواة `sso_clients` فقط. تسجل
  توزيعات المنتجات وحداتها عبر عقد `pkg/nexus` العام في الملف التنفيذي
  النهائي، وتُستدعى داخل العملية نفسها.
- **متجر تطبيقات لكل مستأجر** — صلاحيات التطبيقات والقوائم و RBAC تُدار من
  PostgreSQL (`app_installations`).
- **محلِّل التبعيات** — حل تكراري على رسم بياني موجَّه لا دوري، مع كشف الدورات
  والتحقق من قيود semver.
- **مزامنة الفهرس** — يجلب الإنتاج فهرسًا موقّعًا من `APP_CATALOG_URL`؛ ويستخدم
  التطوير/العمل دون اتصال `catalog/apps.json` كبديل، ثم يزامن البيانات في
  `platform.apps`.

### ٢. المرونة السحابية وتعدد النسخ

</div>

| الوحدة | الغرض |
| --- | --- |
| `internal/kernel/resilience/loadshedder.go` | إسقاط الحمل بـ `503` + `Retry-After` عند الضغط |
| `internal/kernel/cache/bus.go` | إبطال الذاكرة المخبأة بين النسخ عبر Redis مع بديل محلي |
| `internal/kernel/memo/memo.go` | ذاكرة محلية قصيرة الأجل تُبطل بالبادئة لقرارات التفويض |
| `internal/kernel/async/async.go` | goroutines مسماة مع استرداد panic وتسجيل المكدس |

<div dir="rtl">

### ٣. البنية التحتية الرقمية الوطنية

- **XYP — تبادل معلومات الدولة** (`internal/workspace/identity/gerege/xyp.go`): السجل المدني
  للمواطنين (`WS100101`) والتحقق من الكيانات الاعتبارية (`WS100201`).
- **الهوية الرقمية الوطنية E-ID و DAN**
  ([`developer.gerege.mn`](https://developer.gerege.mn)،
  [`eidmongolia.mn`](https://eidmongolia.mn)) — توقيع رقمي بالبنية التحتية
  للمفاتيح العامة، ورمز لمرة واحدة عبر الهاتف، ودخول موحَّد مصرفي، وتحقق
  بيومتري من الوجه.
- **مزوِّد OAuth2 / OIDC مدمج** (`/.well-known/openid-configuration`) يُصدر
  رموز client-credentials للأنظمة الخارجية.
- **تأكيد البريد الإلكتروني** (`internal/workspace/emailverify`) — مسار موحَّد لإثبات ملكية
  عنوان، تستدعيه كل وحدات التطبيقات داخل العملية. ترسل الرسالة الخدمة المستضافة
  (`enigma.mn`)، فلا تحتفظ المنصّة بأي بيانات اعتماد بريد ولا تملك عنوان مُرسِل.
  يُسجَّل التأكيد عند عودة الشخص، وتعمل تلك العودة مرة واحدة فقط. يظهر في
  الإعدادات ← تأكيد البريد الإلكتروني.

> **ملاحظة.** وضع المحاكاة لـ E-ID و DAN و XYP هو تسهيل للتطوير فقط. مع
> `ENVIRONMENT=production` يُعطَّل تلقائيًا، فلا يمكن مطلقًا لرقم تسجيل مُلفَّق
> أن يجتاز المصادقة.

### ٤. مساعد الذكاء الاصطناعي والتحليلات

- **المساعد الذكي** (`internal/workspace/ai/copilot.go`) — محادثة مُصنَّفة حسب النية
  وموصولة ببيانات المستأجر الحية.
- **واجهة توقع المخزون** (`internal/workspace/ai/handlers.go`) — تفوض إلى قدرة
  `stock_forecast` في توزيع مفعّل وتعيد `404` عند عدم وجود مزود.

---

## تطبيقات الأعمال

يحتوي هذا المستودع الأساسي على تطبيق واحد فقط في `catalog/apps.json`.
تسجّل توزيعات المنتجات وحداتها وترحيلاتها الخاصة عبر `pkg/nexus`؛ ولا تُعدّ
تطبيقاتها ميزات موجودة في هذا المستودع.

</div>

| # | التطبيق | المعرِّف | المسار | الوصف |
| --- | --- | --- | --- | --- |
| ١ | عملاء SSO | `io.gerege.nexus.sso_clients` | `/sso-clients` | تسجيل عملاء OAuth2 للأنظمة التي تُسجّل دخول المستخدمين عبر هذه المنصة |

<div dir="rtl">

لا تُفتح المسارات إلا بعد تثبيت التطبيق وتفعيله للمستأجر؛ وإلا فإن البوابة
تُعيد `403 Forbidden`.

---

## بنية المستودع

</div>

```
backend/
  cmd/api/            خادم واجهة HTTP البرمجية (+ بيانات العرض التجريبي)
  cmd/migrate/        مُنفِّذ ترحيلات Goose
  db/migrations/      ترحيلات SQL
  internal/
    kernel/           عناصر تقنية مشتركة بين المستويين
    tenant/           العمل الخاص بمنظمة واحدة
    platform/         تشغيل النشر بأكمله
    apps/             الوحدات التي يحملها هذا التوزيع
  pkg/
    nexus/            SDK عام وعقود للوحدات الخارجية
    platform/         جذر تركيب المستويين
frontend/             عميل الويب Next.js 16 (App Router)
catalog/              فهرس متجر التطبيقات وبياناته الوصفية
deploy/               Dockerfile الإنتاج وإعدادات Nginx
docs/                 التوثيق والترجمات
```

---

<div dir="rtl">

## البدء

### المتطلبات المسبقة

- Go 1.26+
- Node.js 20+
- PostgreSQL 16+ (أو Docker Compose)

### ١. Docker Compose

</div>

```bash
docker compose up -d
```

<div dir="rtl">

تعمل الترحيلات في خدمة `migrate` مخصَّصة تعمل لمرة واحدة قبل بدء الواجهة
البرمجية.

### ٢. يدويًا

**الواجهة الخلفية:**

</div>

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

<div dir="rtl">

**الواجهة الأمامية:**

</div>

```bash
cd frontend
npm ci
npm run dev
```

<div dir="rtl">

افتح [http://localhost:3000](http://localhost:3000).

### بيانات الدخول التجريبية

</div>

| الحقل | القيمة |
| --- | --- |
| البريد الإلكتروني | `admin@example.com` |
| كلمة المرور | `Password123!` |
| المستأجر | `Demo Corporation` (`slug: demo`) |

<div dir="rtl">

يُنشأ حساب العرض التجريبي خارج بيئة الإنتاج فقط. أما في الإنتاج فلا يُنشأ إلا
عند ضبط `SEED_DEMO_DATA=true` صراحةً.

---

## النشر الآلي

كل دفع إلى `main` يُشغِّل [`deploy.yml`](../.github/workflows/deploy.yml):

١. بناء صور الواجهة الخلفية والأمامية ورفعها إلى GHCR (`:latest` و `:<sha>`).

٢. نسخ `docker-compose.prod.yml` إلى الخادم.

٣. كتابة ملف `.env` على الخادم من أسرار GitHub وسحب الصور.

٤. تشغيل الترحيلات حتى اكتمالها، ثم تبديل الواجهة البرمجية والواجهة الأمامية.

٥. فحص `/health` و `/ready`، وطباعة سجلات الحاويات وإفشال التشغيل إذا لم يكن
النشر سليمًا.

للنشر يدويًا: Actions ← *Deploy to Production* ← **Run workflow**، مع إمكانية
تثبيت وسم صورة محدَّد.

الأسرار المطلوبة في المستودع:

</div>

| السر | مطلوب | الوصف |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | نعم | المفتاح الخاص لمستخدم النشر. بدونه يُتخطَّى النشر |
| `POSTGRES_PASSWORD` | نعم | كلمة مرور قاعدة البيانات على الخادم |
| `SSO_DEFAULT_CLIENT_SECRET` | نعم | إلزامي لعميل OAuth2 المدمج في الإنتاج |
| `DEPLOY_HOST` / `DEPLOY_USER` / `DEPLOY_PORT` | لا | الافتراضي `nexus.gerege.mn` / `deploy` / `22` |
| `PUBLIC_ORIGIN` | لا | الافتراضي `https://nexus.gerege.mn` |

<div dir="rtl">

> نطاق الإنتاج هو `nexus.gerege.mn`، الذي حلَّ محل `openerp.gerege.mn` عند
> إعادة التسمية إلى Gerege Nexus. يحدِّد `PUBLIC_ORIGIN` في موضع واحد سياسة
> CORS ومُصدِر OIDC وعنوان استدعاء eID، لذا فإن تغييره يستتبع معه DNS وشهادة
> TLS وكل عميل ثبَّت المُصدِر لديه.

لا يحتاج الخادم سوى Docker — دون شيفرة مصدرية ودون أدوات Go/Node. راجع
[`deploy/.env.prod.example`](../deploy/.env.prod.example) للاطلاع على القيم.

---

## الإعدادات

راجع [`.env.example`](../.env.example) للقائمة الكاملة.

</div>

| المتغيِّر | الافتراضي | الوصف |
| --- | --- | --- |
| `DATABASE_URL` | localhost | سلسلة الاتصال بـ PostgreSQL |
| `PORT` | `8080` | منفذ استماع الواجهة البرمجية |
| `ENVIRONMENT` | `development` | القيمة `production` تُفعِّل الإعدادات المُشدَّدة |
| `APP_CATALOG_PATH` | `catalog/apps.json` | مسار فهرس متجر التطبيقات |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | قائمة المصادر المسموح بها (CORS) |
| `TRUST_PROXY_HEADERS` | `false` | هل يُوثَق بترويسة `X-Forwarded-For` |
| `SEED_DEMO_DATA` | مُفعَّل خارج الإنتاج | إنشاء حساب العرض التجريبي |
| `SSO_DEFAULT_CLIENT_SECRET` | — | مطلوب في الإنتاج |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | مُفعَّل خارج الإنتاج | محاكاة التكاملات الوطنية |

---

<div dir="rtl">

## نظرة عامة على الواجهة البرمجية

</div>

| الطريقة | المسار | الوصف |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | فحوص الحياة والجاهزية |
| `GET` | `/metrics` | مقاييس Prometheus |
| `POST` | `/api/v1/auth/login` | تسجيل الدخول بالبريد وكلمة المرور |
| `POST` | `/api/v1/auth/eid/login` | تسجيل الدخول بالهوية الرقمية الوطنية |
| `POST` | `/api/v1/auth/dan/login` | تسجيل الدخول عبر بوابة DAN |
| `POST` | `/api/v1/auth/logout` | إبطال الجلسة |
| `GET` | `/api/v1/menus` | قوائم التطبيقات المفعَّلة للمستأجر |
| `GET` | `/api/v1/store/apps` | قائمة متجر التطبيقات |
| `POST` | `/api/v1/store/apps/{slug}/install` | تثبيت تطبيق (مسؤول) |
| `POST` | `/api/v1/verify/send` | طلب رابط تأكيد البريد من الخدمة المستضافة |
| `GET` | `/api/v1/verify/landed` | استقبال من أكّد عنوانه — يعمل مرة واحدة فقط |
| `GET` | `/api/platform/v1/email-verifications` | سجل التأكيدات وحالة الخدمة (وحدة التحكم) |
| `POST` | `/oauth2/token` | رمز OAuth2 من نوع client credentials |

<div dir="rtl">

تنتقل رموز الجلسة إما في ملف تعريف ارتباط HttpOnly أو عبر
`Authorization: Bearer <token>`.

---

## الاختبارات وضوابط الجودة

</div>

```bash
# اختبارات وحدات الواجهة الخلفية مع كاشف التسابق
cd backend && go test -race ./...

# التحليل الساكن
cd backend && go vet ./... && golangci-lint run

# فحص الثغرات
cd backend && govulncheck ./...

# بناء الواجهة الأمامية
cd frontend && npm run build
```

<div dir="rtl">

تُشغِّل منظومة التكامل المستمر الفحص اللغوي والاختبارات وبناء الواجهة الأمامية
وبناء صورة Docker و govulncheck و gosec عند كل دفع وكل طلب دمج.

---

## الأمان

- رموز الجلسة قيم عشوائية بطول ٢٥٦ بت، ولا يُخزَّن منها سوى بصمة SHA-256.
- تُجزَّأ كلمات المرور باستخدام bcrypt، ومحاولات تسجيل الدخول محدودة المعدل لكل
  عنوان IP.
- يتطلب تثبيت التطبيقات أو تفعيلها أو تعطيلها وتسجيل التكاملات صلاحيات مسؤول
  المستأجر.
- تستخدم مصادقة عملاء OAuth2 مقارنة ذات زمن ثابت.

أبلغ عن الثغرات وفق ما هو موضَّح في [`SECURITY.md`](../SECURITY.md).

---

## فهرس التوثيق

</div>

| المستند | الوصف |
| --- | --- |
| [مركز التوثيق](README.md) | فهرس كل المستندات والترجمات |
| [البنية المعمارية](ARCHITECTURE.md) | المستويان، المخططات الثلاثة، وعزل البيانات |
| [كتابة وحدة](MODULES.md) | عقد `pkg/nexus` وكيف يصل التطبيق إلى النشر |
| [المساهمة](../CONTRIBUTING.md) | سير عمل المساهمة |
| [سياسة الأمان](../SECURITY.md) | الإبلاغ عن الثغرات |
| [مدونة السلوك](../CODE_OF_CONDUCT.md) | معايير المجتمع |
| [سجل التغييرات](../CHANGELOG.md) | تاريخ الإصدارات |

---

<div dir="rtl">

## الشكر ومصادر الإلهام

١. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
من **[@snykk](https://github.com/snykk)** — أسس واجهة REST البرمجية بلغة Go.

٢. **[Odoo](https://github.com/odoo/odoo)** — متجر التطبيقات المعياري ونموذج
التبعيات.

٣. **[go-zero](https://github.com/zeromicro/go-zero)** — محرك المرونة السحابي
المنشأ.

---

## الترخيص

حقوق النشر (c) 2026 **Gerege Systems Development Team, Gerege Nomadica Foundation**. يُوزَّع بموجب رخصة Apache 2.0 — راجع [`LICENSE`](../LICENSE).

أيقونات الأعلام من [Flaticon](https://www.flaticon.com/)
([بيان الإسناد](assets/icons/ATTRIBUTION.md)).

</div>
