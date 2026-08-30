# 0005 — Нэг бинарь, хоёр хаалга: урсгал бүр өөрийн origin-той

- **Огноо:** 2026-08-24
- **Байдал:** Хүлээн авсан, хэрэгжсэн.
- **Холбоотой:** `TWO_PLANES_PROPOSAL.md`;
  `TWO_PLANES_REVIEW.md`;
  [`CONTROL_PLANE.md` §2](../CONTROL_PLANE.md);
  `deploy/nginx/cp.nexus.gerege.mn.conf`,
  `deploy/nginx/snippets/cp-allowlist.conf`,
  `internal/platform/operator/middleware.go` (`HostGate`),
  `internal/kernel/security/csrf.go`, `frontend/proxy.ts`

## Асуулт

Хоёр урсгалын ажил дуусахад зүй ёсны асуулт гарлаа: **backend нэг болсон
байхад `cp.nexus.gerege.mn` яагаад тусдаа хэвээр байна вэ?** Нэг хост дээр
хоёр нэвтрэх дэлгэцтэй байгаад цаашаа явж болохгүй гэж үү.

Асуулт зөв тавигдсан. `TWO_PLANES_PROPOSAL.md` өөрөө «`cp` гэдэг тусдаа зүйл
байхаа болино» гэж бичсэн бөгөөд тэр амлалт биелсэн: `controlplane` package
алга болж, урсгал нь `internal/platform` болж домэйнээр задарсан; нэг
`cmd/api`, нэг image, нэг deploy. Тэгэхээр хоёр hostname нь хуучин
хэлбэрийн үлдэгдэл мэт харагдана.

## Юу олдсон бэ

**Хост бол кодын хуваалт биш.** Тэр бол хөтчийн origin — өөрөөр хэлбэл
backend-ийн schema, grant, import тест гурав нь серверийн талд хийж байгаа
зүйлийг **хөтчийн талд** хийдэг цорын ганц механизм. Кодыг нэгтгэсэн нь
origin-ыг нэгтгэх шалтгаан болохгүй; хоёр нь өөр давхаргын хил.

Нэг хост болговол алдагдах гурван зүйл, гуравуулаа энэ репод бичигдсэн байна.

### 1. Allowlist нь server түвшнээс location түвшин рүү бууна

`cp.nexus.gerege.mn.conf` нь `snippets/cp-allowlist.conf`-ыг **server
түвшинд** оруулдаг, мөн яагаад гэдгээ өөрөө хэлсэн:

> *Placed at server level so it covers the frontend, the API and anything
> either of them grows later — a per-location copy is one location away from
> a hole.*

Нэг хост дээр тэр заавал хуваагдана: `location /cp/`, `location
/api/platform/v1/`, мөн маргааш консол шинэ зам ургуулах бүрд нэг мөр. Файл
өөрөө нэрлэсэн бүтэлгүйтлийн хэлбэр рүү шилжинэ.

### 2. Консол «газрын зураг дээр байхгүй» чанараа алдана

`cp-allowlist.conf`-ийн амлалт:

> *nothing about the console — not its login screen, not its existence — is
> visible from an address that is not listed.*

`HostGate` нь 403 биш **404** буцаадаг бөгөөд `middleware.go` шалтгааныг нь
бичсэн: *«a 403 would confirm that something is there. The console is not a
locked door on a public street, it is an address that is not on the map.»*

Нэг хост дээр эдгээрийн аль нь ч утгагүй болно. Юуг юунаас нь ялгах вэ —
`Host` толгой хоёр тохиолдолд ижил ирнэ. Операторын нэвтрэх дэлгэц нь
интернэтээс олдох хуудас болж, операторууд нэрлэсэн байг болно.

### 3. Хамгийн ноцтой нь: `cp_session` тенантын бүх хүсэлттэй хамт явна

`internal/kernel/security/csrf.go` хоёр cookie-г нэрлэдэг:
`TenantSessionCookie = "session_token"`, `ControlPlaneSessionCookie =
"cp_session"`. Өнөөдөр тэдгээр нь **өөр origin** дээр амьдардаг тул хөтөч
өөрөө тэднийг хооронд нь хаадаг — ямар ч код бичих шаардлагагүйгээр.

Нэг origin болбол `cp_session` нь тенантын origin руу явах бүх хүсэлттэй
хамт явна. Тэр origin дээр юу байдаг вэ: OAuth2 provider-ийн зөвшөөрлийн
дэлгэц, SSO federation-ы callback, Google-ийн холболт, eID/ДАН-ы гарын
үсгийн урсгал, AI чат, төхөөрөмжийн шугамууд. Өөрөөр хэлбэл **гуравдагч
талын оролт хүлээж авдаг** хуудсууд.

Нэмээд `nexus.gerege.mn.conf`-д Content-Security-Policy **санаатай
байхгүй**, шалтгааныг нь тэр файл өөрөө бичсэн (*«A policy worth having has
to be written against what the Next.js build actually emits»*) бөгөөд тэр нь
зөв шийдвэр. Гэхдээ үр дүн нь: тэдгээр хуудсын аль нэг дээрх XSS нь
операторын session болно, түүнийг зогсоох давхарга байхгүй.

`Path=/cp` нь энэ асуудлын хариу биш. Path бол origin биш: ижил origin дээр
ажиллаж байгаа скрипт `/cp` руу `fetch` хийхэд cookie дагана.

### 4. Оронд нь юу авах вэ — бараг юу ч биш

Салангид хост нь нэг DNS бичлэг, нэг сертификат (`certbot` нь
`CONTROL_PLANE.md` §3.1-д аль хэдийн скриптлэгдсэн), нэг nginx conf-оор
төлдөг. Deploy нь **аль хэдийн нэг**: хоёр server block ижил upstream руу
заана (`127.0.0.1:8082` ба `:3008`), ижил бинарь хариулна.

Мөн: нэг хост дээр хоёр нэвтрэх дэлгэц гэдэг нь **нэвтрэлт хоёр хэвээр**
гэсэн үг. Оператор ямар ч тохиолдолд хоёр удаа нэвтэрнэ, учир нь
`platform.operator_accounts` нь `platform.users` биш — тэр ялгаа нь хилийн
бүтэн утга. Тэгэхээр нэгтгэлээс гарах хэрэглэгчийн ашиг тэг.

## Шийдвэр

**Урсгал бүр өөрийн origin-той байна.** `nexus.gerege.mn` нь тенантын
урсгалынх, `cp.nexus.gerege.mn` нь платформынх. Хоёуланг нь **нэг бинарь,
нэг image, нэг deploy, нэг upstream** үйлчилнэ; hostname нь аль хаалга
болохыг сонгоно.

Зөв томьёолол нь: **нэг backend, хоёр хаалга.**

Хоёр урсгалын хил дөрвөн давхаргад татагдана, тус бүр өөр зүйлээс
хамгаална:

| Давхарга | Хаана | Юунаас |
| --- | --- | --- |
| Import | `internal/planes_test.go` | Кодын холбоо |
| Schema, grant | `00079_two_schemas`, `schema_split_test.go` | Query |
| Session, role | `dbguard`, `operator.RequireOperator` | Эрх |
| **Origin** | **hostname, cookie, nginx** | **Хөтөч** |

Дөрөв дэх мөрийг устгах нь бусад гурвыг нь хүчингүй болгохгүй — гэхдээ
хөтчийн талд ямар ч хамгаалалт үлдэхгүй.

## Энэ шийдвэр юу гэж хэлээгүй вэ

- **`cp` кодод буцаж ирэхгүй.** `controlplane` package алга болсон нь
  хэвээр. Hostname бол deployment-ийн зохион байгуулалт, package биш.
- **UI-ийн `/cp` зам нь implementation detail.** Browser `cp.nexus.gerege.mn/`
  рүү ороход frontend proxy 308-аар `/cp` руу оруулна. Next.js route `/cp`
  хэвээр байгаа нь origin-ийн шийдвэрийг өөрчлөхгүй.
- **Хоёр нэвтрэлт нь алдагдал биш.** Оператор ба тенантын гишүүн хоёр
  өөр биеийн байцаалт — санаатай. Операторын тенант руу орох audit-тай зам
  бол impersonation (шалтгаан, 30 минут, banner, хоёр талын бүртгэл), хоёр
  дахь нэвтрэлт биш.

## Хэрэгжилт

Шийдвэртэй хамт илэрсэн хоёр дутагдал хоёулаа засагдсан:

1. `frontend/proxy.ts` нь `CONTROL_PLANE_HOST`-ыг шалгана. Control host дээр
   `/` → 308 `/cp`; `/cp/*` ба `/api/platform/v1/*` нээлттэй; бусад зам 404.
   Tenant host дээр `/cp/*` 404. `app/cp/layout.tsx` server component давхар
   gate хэвээр.
2. `.env.example` болон `docker-compose.yml` development-д
   `CONTROL_PLANE_HOST=admin.localhost` ашиглаж, tenant/control frontend ба API-г
   тусдаа origin-оор ажиллуулна.

Boundary-г дараах тестүүд хамгаална:

- `frontend/tests/control-plane-host.test.mjs` — host/path decision;
- `frontend/scripts/check-control-plane-host.mjs` — static wiring;
- `frontend/scripts/smoke-control-plane-host.mjs` — production build дээрх
  root redirect болон хоёр талын 404;
- production deploy smoke — TLS vhost-ын `/` redirect, canonical API HostGate,
  legacy `/cp/api` redirect.

## Хэзээ эргэж харах вэ

Энэ шийдвэрийг дараах хоёр тохиолдолд дахин нээнэ:

- **Операторууд тогтмол хаяггүй болбол.** `cp-allowlist.conf` өөрөө үүнийг
  урьдчилан хэлсэн: *«If your operators do not have fixed addresses, this
  file is the wrong control and a VPN or an identity-aware proxy is the right
  one.»* Тэр өдөр 1-р үндэслэл суларна — гэхдээ 2, 3 хэвээр.
- **Тенантын origin дээр хатуу CSP бичигдвэл.** 3-р үндэслэл суларна.
  Гэхдээ CSP бол хамгаалалт, origin бол хил: CSP-тэй болсон нь хоёр session
  нэг origin-д амьдрах шалтгаан болохгүй.

Аль ч тохиолдолд шийдвэрийг өөрчлөхийн өмнө энэ баримтын гурван үндэслэл
тус бүрд юу солигдсоныг бичих ёстой.

## Татгалзсан хувилбар: нэг хост, хоёр нэвтрэх дэлгэц

| | Авах | Алдах |
| --- | --- | --- |
| Дэд бүтэц | 1 DNS бичлэг, 1 сертификат, 1 nginx conf | — |
| Deploy | — (аль хэдийн нэг) | — |
| Нэвтрэлт | — (хоёр хэвээр) | — |
| Allowlist | — | server түвшнээс location түвшин рүү |
| Илрүүлэлт | — | Консолын нэвтрэх дэлгэц олдоцтой болно |
| Cookie | — | `cp_session` тенантын бүх хүсэлттэй явна, CSP-гүй origin дээр |

Гурван мөр алдаж, нэг мөр авна.
