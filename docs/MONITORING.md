# Мониторинг — ажиглалтын стек

Gerege Nexus-ийн хэмжүүр, лог, дохиоллын систем: юу хаана байдаг, яаж асаах,
шинэ хэмжүүр яаж нэмэх.

[Баримт бичгийн төв рүү буцах](README.md) ·
[Дизайны санал](MONITORING_AND_REPORTING_PROPOSAL.md) ·
[Runbook-ууд](RUNBOOKS.md)

---

## 1. Юу байдаг вэ

Стек нь **үндсэн платформоос бүрэн тусдаа** compose файлд байна. Энэ нь
санамсаргүй биш: мониторинг бүхэлдээ унасан ч платформ мэдэхгүй, ямар ч
хүсэлтийн зам дээр эдгээрийн нэг нь ч байхгүй, `down` хийх нь ямар ч цагт
аюулгүй үйлдэл.

| Компонент | Контейнер | Порт (loopback) | Үүрэг |
| --- | --- | --- | --- |
| Prometheus | `gerege_nexus_prometheus` | 9091 | Хэмжүүр, 60 хоног (20 GB тааз) |
| Alertmanager | `gerege_nexus_alertmanager` | 9093 | Групплэх, дарах, илгээх |
| Loki | `gerege_nexus_loki` | 3100 | Лог, 31 хоног |
| Alloy | `gerege_nexus_alloy` | 12345 | Docker логийг Loki руу |
| Tempo | `gerege_nexus_tempo` | 3200, 4318 | Trace, 72 цаг |
| Grafana | `gerege_nexus_grafana` | 3009 | Dashboard, лог, trace |
| node_exporter | `gerege_nexus_node_exporter` | 9100 | Хостын CPU, RAM, диск |
| cAdvisor | `gerege_nexus_cadvisor` | 8085 | Контейнер бүрийн хэрэглээ |
| postgres_exporter | `gerege_nexus_postgres_exporter` | 9187 | `pg_stat_*` |
| redis_exporter | `gerege_nexus_redis_exporter` | 9121 | Redis |

Бүх порт **зөвхөн 127.0.0.1** дээр bind хийгдсэн. Гаднаас хандах ганц зам нь
nginx-ээр гарсан Grafana (§4) эсвэл SSH tunnel.

### Хэмжүүр хаанаас ирдэг вэ

Платформ өөрөө `/metrics` дээр дараахыг гаргана
(`backend/internal/kernel/telemetry/`):

| Хэмжүүр | Тайлбар |
| --- | --- |
| `http_server_request_duration_seconds{http_request_method,http_route,http_response_status_code}` | OpenTelemetry-ийн HTTP semantic convention. `http_route` нь chi-ийн routed pattern — түүхий URL биш. **Хүсэлтийн тоо нь энэ гистограммын `_count` цуврал** — тусад нь counter байхгүй |
| `pgxpool_*` | Холболтын pool: эзлэгдсэн, сул, нийт, хүлээлт |
| `db_client_operation_duration_seconds{pgx_operation_type}` | Өгөгдлийн сангийн үйлдлийн хугацаа: query, prepare, acquire, connect. Үүнийг otelpgx гаргадаг ба **зөвхөн trace асаалттай үед** гарна — meter provider үүсэхэд тэр өөрөө бүртгүүлдэг |
| `external_request_duration_seconds{system,operation,status}` | ХУР, eID, ДАН, eSign, Gemini, и-мэйл баталгаажуулалт |
| `logins_total{method,result}` | password, eid, dan, google, sso |
| `invoices_created_total` | |
| `documents_signed_total{rail,result}` | rail: EID, DAN, HSM |
| `ai_requests_total{kind}` | copilot, chat, stt, tts, translate, forecast |
| `resilience_load_shed_total`, `resilience_in_flight_requests` | |
| `resilience_retry_total{name}` | |
| `go_*`, `process_*` | client_golang-ийн бэлэн collector-ууд |
| `target_info` | Энэ суулгацын `service_name`, `service_version`, `deployment_environment_name` — OTel resource |

Бүх хэмжүүр **OpenTelemetry metrics SDK**-аар дамжиж, Prometheus exporter-ээр
`/metrics` дээр гарна (`internal/kernel/telemetry/otelmetrics.go`). Instrument-
ийн нэр цэгтэй (`http.server.request.duration`), Prometheus дээр гарахдаа
доогуур зураастай, нэгжийн болон `_total` дагаваржинэ. Exporter нь
client_golang-ийн үндсэн registry дээр суудаг тул `go_*`, `process_*` хамт нэг
endpoint дээр үлдэнэ.

**Trace-тай холбоос — exemplar.** Гистограммын сэмпл бүр trace_id авч явна
(`--enable-feature=exemplar-storage` Prometheus дээр асаалттай). Grafana-ийн
latency график дээрх удаан цэг дээр дарвал яг тэр удаашруулсан trace нээгдэнэ.

Энэ нь **trace асаалттай үед л** ажиллана: сэмпл нь бичигдсэн контекст нь
sample хийгдсэн span-д харьяалагдаж байж trace_id авч явдаг. §11-ийг үз.
2026-08-29-нд бүх гинжийг шалгасан: exemplar → Prometheus-ийн
`/api/v1/query_exemplars` → Tempo дахь trace, дотор нь `query SELECT`,
`pool.acquire` спанууд.

**Аль ч label-д тенант байхгүй.** Тенант ID эсвэл slug нь label болвол
time series-ийн тоо байгууллагын тоогоор үржинэ, гарсан байгууллагын series нь
retention дуустал үлдэнэ. Тенантаар задарсан тоог тайлангийн модулиас авна —
тэнд мөр устгаж болдог, time series-д тэр боломж байхгүй.

---

## 2. Босгох

### 2.1 Урьдчилсан нөхцөл

Платформын стек ажиллаж байх ёстой (`docker-compose.prod.yml`), миграцууд
**00044** хүртэл хийгдсэн байх ёстой.

### 2.2 Нэг удаагийн алхам — өгөгдлийн сангийн нууц үг

Миграц 00044 нь `monitoring` гэсэн role-ыг `pg_monitor` эрхтэй, **нууц үггүй**
үүсгэдэг. Нууц үг нь репод байх боломжгүй тул оператор нэг удаа өгнө:

```bash
PASSWORD=$(openssl rand -base64 24)
docker exec -i gerege_nexus_postgres psql -U postgres -d platform_db \
  -c "ALTER ROLE monitoring WITH PASSWORD '$PASSWORD'"
echo "MONITORING_DB_PASSWORD=$PASSWORD"
```

Гарсан утгыг `.env.monitoring`-д бичнэ.

Энэ role нь `pg_stat_*` уншина, өөр юу ч биш — ямар ч хүснэгт, ямар ч тенантын
мөрийг харахгүй. Exporter-ыг `postgres` superuser-ээр ажиллуулах нь хамгийн
түгээмэл алдаа: тэр үед мониторингийн контейнер эвдрэлд орох нь өгөгдлийн
сангийн бүрэн хандалт алдагдсантай тэнцэнэ.

### 2.3 Орчин

```bash
cd /opt/open-gerege-nexus
cp deploy/.env.monitoring.example .env.monitoring
# GRAFANA_ADMIN_PASSWORD ба MONITORING_DB_PASSWORD хоёрыг бөглөнө
chmod 600 .env.monitoring
```

### 2.4 Асаах

```bash
docker compose -f deploy/docker-compose.monitoring.yml \
  --env-file .env.monitoring up -d
```

Шалгах:

```bash
# Бүх scrape target "up" эсэх
curl -s localhost:9091/api/v1/targets | jq '.data.activeTargets[] | {job:.labels.job, health}'

# Alertmanager ямар сувагтай болсон
docker logs gerege_nexus_alertmanager 2>&1 | grep 'notification channels'

# Loki лог хүлээж авч байгаа эсэх
curl -s 'localhost:3100/loki/api/v1/labels' | jq
```

### 2.5 Унтраах

```bash
docker compose -f deploy/docker-compose.monitoring.yml down
```

Өгөгдөл нэрлэсэн volume-д үлдэнэ (`gerege_nexus_monitoring_*`). `down -v` нь
60 хоногийн хэмжүүр, 31 хоногийн логийг устгана — буцаах арга байхгүй.

---

## 3. Сүлжээ

Мониторингийн стек нь платформын Docker сүлжээнд **гаднаас нэгдэнэ**
(`platform` network, `NEXUS_NETWORK`). Ингэснээр Prometheus нь
`gerege_nexus_backend:8080`-ыг, postgres_exporter нь `gerege_nexus_postgres`-ыг
контейнерийн нэрээр шууд дуудна — хостын порт руу тойрохгүй.

Сүлжээний нэр нь платформын стек байрлах хавтаснаас гаралтай. Өөр бол:

```bash
docker network ls | grep nexus
# ...дараа нь .env.monitoring дотор NEXUS_NETWORK=<нэр>
```

---

## 4. Grafana руу орох

**SSH tunnel (санал болгож буй).** Нэмэлт хандалтын гадаргуу үүсгэхгүй:

```bash
ssh -L 3009:127.0.0.1:3009 <server>
# дараа нь http://localhost:3009
```

**Өөрийн домэйнээр — `https://monitor.nexus.gerege.mn/`.** Осол 03:00 цагт
болдог бөгөөд тэр үед хүн гартаа утас барьж байдаг. SSH tunnel тэр мөчид
хэрэггүй.

```bash
sudo cp deploy/nginx/monitor.nexus.gerege.mn.conf /etc/nginx/sites-available/monitor.nexus.gerege.mn
sudo ln -s /etc/nginx/sites-available/monitor.nexus.gerege.mn /etc/nginx/sites-enabled/
sudo htpasswd -c /etc/nginx/.htpasswd-nexus-monitoring <хэрэглэгч>   # Alertmanager-т
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d monitor.nexus.gerege.mn
```

`*.nexus.gerege.mn` DNS нь wildcard — шинэ бичлэг хэрэггүй. Гэрчилгээ нь
wildcard **биш**, тул certbot-ыг заавал ажиллуулна.

Гарах хаягууд: `/grafana/` — Grafana, `/alertmanager/` — Alertmanager
(basic auth-ын цаана), бусад бүхэн 404. Домэйныг дангаар нь бичихэд Grafana
руу шилжинэ. Prometheus, Loki, Tempo, cAdvisor, exporter-ууд **гарахгүй** —
Prometheus-д ямар ч нэвтрэлт байхгүй бөгөөд түүний query API нь асуусан хүн
бүрт энэ суулгацын бүх бүтцийг задлан хэлнэ.

Хуучин арга — `nexus.gerege.mn/grafana/` — `snippets/nexus-monitoring.conf`-оор
хэвээр ажиллана. Тусдаа домэйн байхгүй суулгацуудад зориулсан.

### Платформын бүртгэлээр нэвтрэх

Суулгац өөрөө identity provider. Grafana-д тусдаа нууц үг хадгалахын оронд
`GRAFANA_OAUTH_*`-ыг бөглөвөл операторууд байгаа бүртгэлээрээ орно, **суулгацыг
анх тохируулсан админ нь Grafana-ийн server administrator болно**.

1. Платформ дээр клиент бүртгэнэ — Хөгжүүлэгч → Аппликейшн:
   - redirect_uris — `https://monitor.nexus.gerege.mn/grafana/login/generic_oauth`
   - grant_types — `authorization_code`, `refresh_token`
   - scopes — `openid`, `profile`, `email`, `roles`
2. Тэр host платформын `OAUTH_REDIRECT_HOSTS`-д байх ёстой. Тэр жагсаалт
   дэд домэйныг өвлүүлдэггүй — `monitor.nexus.gerege.mn` бүтнээрээ бичигдэнэ.
   Утга нь GitHub-ийн repository **variable** (Settings → Variables), учир нь
   `.env`-ийг deploy бүр GitHub-ээс шинээр бичдэг: серверт гараар нэмсэн мөр
   дараагийн deploy хүртэл л амьдарна.

   ```
   gh variable set OAUTH_REDIRECT_HOSTS -b "nexus.gerege.mn,monitor.nexus.gerege.mn"
   ```
3. `.env.monitoring` дотор `GRAFANA_OAUTH_ENABLED=true`, `GRAFANA_OAUTH_CLIENT_ID`,
   `GRAFANA_OAUTH_CLIENT_SECRET`-ийг тавиад Grafana-г дахин эхлүүлнэ.

Эрх нь `roles` scope дээрх `platform_admin` claim-ээр шийдэгдэнэ. Тэр нь
**эхний байгууллагын** admin — өөрөөр хэлбэл тохиргооны шидтэн (setup wizard)
үүсгэсэн байгууллагын админ — гагцхүү тэр хүнд үнэн. Өчигдөр бүртгүүлсэн
байгууллагын админ ч гэсэн Viewer болж орно. Яагаад тэр ялгааг платформ талд
тавьсныг `internal/workspace/ssoprovider/endpoints.go` дотор бичсэн.

Claim нь *хүний* тухай — токен аль workspace-д зориулагдсанаас хамаарахгүй.
Хувийн workspace-даасаа нэвтэрсэн ч мөн адил.

**Хэн болохыг шалгах:**

```sql
SELECT u.email
  FROM registry.users u
  JOIN workspace.memberships m ON m.user_id = u.id
  JOIN workspace.membership_roles mr ON mr.membership_id = m.id
  JOIN workspace.roles r ON r.id = mr.role_id AND r.code = 'admin'
 WHERE m.tenant_id = (SELECT id FROM registry.tenants
                       WHERE kind = 'organisation' ORDER BY created_at, id LIMIT 1);
```

eID-ээр нэвтэрсэн бүртгэл нь и-мэйлээр нэвтэрсэн бүртгэлээс **өөр данс** байж
болно: eID-ийн хаяг нь Gerege дугаараас гардаг (`10000263@gemail.com` хэлбэртэй).
Grafana-д админ болохгүй байгаагийн хамгийн түгээмэл шалтгаан нь энэ бөгөөд
дээрх query хариуг нь шууд хэлнэ.

`GRAFANA_ADMIN_PASSWORD` хэвээр ажиллана — энэ бол буцах зам. Identity provider
унасан үед мониторингийн стек нээгдэхгүй байх нь яг түүнийг хамгийн их
хэрэгтэй мөч.

Dashboard-ууд **"Gerege Nexus"** гэсэн хавтсанд өөрөө үүснэ:

| Dashboard | Хэзээ харах |
| --- | --- |
| **API тойм** | "Ямар нэг зүйл эвдэрсэн үү" — эхний дэлгэц |
| **Гадаад системүүд** | Асуудал бидний тал уу, тэдний тал уу |
| **Инфраструктур** | Удаашрал — хост, контейнер, Postgres, Redis |
| **Тэсвэрлэлт ба эзлэхүүн** | Ачаалал, pool, бизнесийн тоо |
| **Логууд** | Хүсэлтийн мөрүүд, алдаа, audit — түвшин ба чөлөөт текстээр |
| **Аюулгүй байдал ба хандалт** | Консолын нэвтрэлт, break-glass, эрхийн татгалзал |
| **Мониторингийн эрүүл мэнд** | Ажиглагчийг хэн ажиглах вэ |

---

## 5. Dashboard-as-code

Dashboard-ууд нь `deploy/monitoring/grafana/dashboards/*.json` дотор,
provisioning-оор ачаалагдана. `allowUiUpdates: false` — **браузераас хийсэн
засварыг хадгалж болохгүй**.

Энэ нь төвөгтэй боловч зориудаар: 02:00 цагт ослын үеэр хэн нэгний засварласан
панел үнэ цэнэтэй бөгөөд түүнийг хадгалах арга нь commit, дараагийн
`docker compose down -v` устгачих volume доторх мөр биш.

Засварлах урсгал:

1. Grafana дотор панелаа засна;
2. Dashboard → **Export → Save to file** (эсвэл JSON Model-ыг хуулна);
3. Репо доторх файлыг солино;
4. Grafana хавтсыг 30 секунд тутам дахин уншдаг тул серверт файл шинэчлэхэд
   restart шаардлагагүй.

Datasource-ийн `uid` (`nexus-prometheus`, `nexus-loki`) нь тогтмол.
Өөрчилвөл тэдгээрийг нэрлэсэн панел бүр чимээгүй эвдэрнэ.

---

## 6. Лог хайх

Loki-д **label-ууд нь индекс**: `container`, `service`, `level`, `deployment`,
`job`. Лог мөрийн доторх зүйл индексжээгүй бөгөөд query үед шүүгдэнэ.

```logql
# Backend-ийн бүх алдаа — level нь label тул энэ нь индексээр шүүгдэнэ
{container="gerege_nexus_backend", level="error"}

# Нэг хүсэлтийн бүх мөр — request_id нь хариуны X-Request-Id толгойд байдаг
{container="gerege_nexus_backend"} | json | request_id = "abc123"

# Нэг байгууллагын үйлдлүүд
{container="gerege_nexus_backend"} | json | tenant_id = "<uuid>"

# Audit-ийн мөрүүд
{container="gerege_nexus_backend"} | json | msg = "AUDIT_EVENT"
```

`request_id`, `tenant_id`-г **label болгож болохгүй**. Тэдгээр нь Loki-г
өөрийнхөө зайлсхийхийг зорьсон бүрэн текстийн индекс болгож хувиргана —
утга бүр тусдаа stream үүсгэнэ.

---

## 7. Шинэ хэмжүүр нэмэх

1. **Cardinality-г эхэлж бод.** Label бүрийн боломжит утгын тоог үржүүл.
   Тенант, хэрэглэгч, ID, чөлөөт текст, түүхий зам — эдгээрийн аль нь ч
   label болохгүй. Утгын багц нь кодод бичигдсэн тогтмол байх ёстой.
2. Хэмжүүрийг `internal/kernel/telemetry` дотор `mustCounter`,
   `mustUpDownCounter` эсвэл `mustHistogram`-аар пакетын түвшний хувьсагч
   болгож зарла (`business.go`, `external.go`, `resilience.go` жишээ).
   `init()`-д бүртгэх шаардлагагүй — эдгээр нь OpenTelemetry-ийн global meter
   дээр үүсдэг ба `SetupMetrics` дуудагдахад автоматаар жинхэнэ provider руу
   холбогдоно.

   **Нэрийг цэгээр бич** — `resilience.load_shed` шиг. Prometheus дээр гарахдаа
   доогуур зураас болж, counter бол `_total`, нэгж заасан бол нэгжийн дагавар
   нэмэгдэнэ: `resilience_load_shed_total`. Semantic convention байгаа зүйлд
   (HTTP, DB, RPC) өөрийн нэр бүү зохио — OpenTelemetry-ийн нэрийг ашигла.
3. Нэмэгдүүлэх дуудлагыг **бүх зам нийлдэг ганц цэгт** тавь — handler бүрт
   биш. Жишээ: Google-ийн бүх татгалзал `failGoogle`-аар, гарын үсгийн хоёр
   rail `store.markSigned`-аар өнгөрдөг.
4. Тест бич: `instrumentation_test.go` доторх `counterValue` туслах
   функцүүдийг ашигла.
5. Хэрэгтэй бол dashboard JSON-д панел нэм, alert дүрэм нэмбэл
   `RUNBOOKS.md`-д runbook **заавал** бич.

Гадаад систем нэмэх бол `external.go` доторх тогтмолд нэрийг нь нэмнэ —
`knownSystems`-д байхгүй нэр нь `other` болж нугалагдана, энэ нь label-ын
багцыг тогтмолгүй өргөжихөөс хамгаалдаг санаатай зан төлөв.

---

## 8. TLS-ийн хугацаа

Сертификат нь хостын nginx дээр байдаг тул түүнийг хэмжих ганц зам нь
node_exporter-ийн textfile collector. Дараах cron ажлыг **хост дээр** тавина
(blackbox exporter нэмэхгүй байх шалтгаан: сертификат нь энэ л хост дээр
байгаа, гаднаас шалгах ажлыг Uptime Kuma §9 хийнэ):

```bash
sudo mkdir -p /var/lib/node_exporter
sudo tee /usr/local/bin/nexus-tls-expiry.sh >/dev/null <<'EOF'
#!/bin/sh
# TLS-ийн дуусах хугацааг node_exporter-ийн textfile collector-т бичнэ.
# Атомик бичилт: node_exporter хагас бичигдсэн файлыг уншиж болохгүй.
set -eu
OUT=/var/lib/node_exporter/nexus_tls.prom
TMP=$(mktemp)
echo '# HELP nexus_tls_not_after_timestamp_seconds Certificate expiry, unix seconds' > "$TMP"
echo '# TYPE nexus_tls_not_after_timestamp_seconds gauge' >> "$TMP"
for dir in /etc/letsencrypt/live/*/; do
    domain=$(basename "$dir")
    [ -f "$dir/cert.pem" ] || continue
    end=$(openssl x509 -enddate -noout -in "$dir/cert.pem" | cut -d= -f2)
    epoch=$(date -d "$end" +%s)
    echo "nexus_tls_not_after_timestamp_seconds{domain=\"$domain\"} $epoch" >> "$TMP"
done
mv "$TMP" "$OUT"
chmod 644 "$OUT"
EOF
sudo chmod +x /usr/local/bin/nexus-tls-expiry.sh
sudo sh -c 'echo "17 4 * * * root /usr/local/bin/nexus-tls-expiry.sh" > /etc/cron.d/nexus-tls-expiry'
sudo /usr/local/bin/nexus-tls-expiry.sh
```

Энэ ажил ажиллахгүй бол `NexusTLSExpiryUnknown` дохио өгнө — "хэмжилт байхгүй"
нь "бүх зүйл хэвийн"-тэй яг адилхан харагддаг гэдгийг сануулах зорилготой.

---

## 8а. Нөөцлөлт

Нөөцлөлт нь **хост бүр дээр гараар суудаг**, TLS-ийн ажилтай яг ижил — deploy
нь үүнийг хийхгүй. 2026-08-30-нд nexus.gerege.mn дээр шалгахад cron суугаагүй,
скрипт хостод байхгүй, `platform_backups` хүснэгтэд нэг ч мөр байгаагүй.
Консолын Нөөцлөлт дэлгэц хоосон байсныг хэн ч анзаараагүй.

```bash
sudo install -m 755 /opt/open-gerege-nexus/deploy/scripts/backup.sh \
    /usr/local/bin/nexus-backup.sh
sudo mkdir -p /var/backups/gerege-nexus /var/lib/node_exporter
sudo sh -c 'echo "15 3 * * * root /usr/local/bin/nexus-backup.sh >> /var/log/nexus-backup.log 2>&1" \
    > /etc/cron.d/nexus-backup'
sudo /usr/local/bin/nexus-backup.sh          # эхнийхийг нь одоо ажиллуул
```

Скрипт нь гурван зүйлийг хийнэ: `pg_dump` авах, хуучныг цэвэрлэх, үр дүнг
**хоёр газар** бүртгэх — `platform_backups` (консол уншина) ба node_exporter-ийн
textfile (Prometheus уншина). Хоёр дахь нь шөнө дунд хэн нэгэнд сэрэмжлүүлэг
илгээж чадах цорын ганц хувилбар:

| Хэмжүүр | Утга |
| --- | --- |
| `nexus_backup_last_run_timestamp_seconds` | Хамгийн сүүлд ажилласан мөч — амжилттай эсэхээс үл хамаарна |
| `nexus_backup_last_success_timestamp_seconds` | Хамгийн сүүлд **амжилттай** болсон мөч |
| `nexus_backup_last_size_bytes` | Сүүлийн амжилттай dump-ын хэмжээ |
| `nexus_backup_last_ok` | 1 эсвэл 0 |

Гурван дохио: `NexusBackupNeverSeen` (метрик огт байхгүй — скрипт суугаагүй),
`NexusBackupStale` (26 цагаас удсан), `NexusBackupFailing` (ажилласан ч
бүтээгүй). Эхнийх нь хамгийн чухал: "нөөцлөлт унасан"-аас "нөөцлөлт байгаа
эсэхийг хэн ч мэдэхгүй" нь илүү муу.

### 8б. Өөр байршил — backups.nexus.gerege.mn

Дискэн дээрх нөөцлөлт нь тэр дискийг алдвал хамт алга болно. `backups.*` нь
S3-той нийцэх сан (MinIO), суулгац бүр өөрийн шифрлэгдсэн dump-аа тийш түлхэнэ.

```bash
cp deploy/.env.backups.example /opt/open-gerege-nexus/.env.backups   # утгуудыг бөглө
cd /opt/open-gerege-nexus
docker compose -f deploy/docker-compose.backups.yml --env-file .env.backups up -d
sudo cp deploy/nginx/backups.nexus.gerege.mn.conf /etc/nginx/sites-available/backups.nexus.gerege.mn
sudo ln -s /etc/nginx/sites-available/backups.nexus.gerege.mn /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d backups.nexus.gerege.mn
```

**Гурван шинж чанар, ач холбогдлын дарааллаар:**

1. **Шифрлэгдсэн байдлаар ирнэ.** Эх хостод зөвхөн age-ийн *нийтийн* түлхүүр
   байна, тиймээс эвдэрсэн платформ өөрийн илгээсэн зүйлээ уншиж чадахгүй, санг
   барьсан хэн ч уншиж чадахгүй. Хувийн түлхүүр нь операторт байна.

   ```bash
   apt-get install -y age
   age-keygen -o /root/backups-age-key.txt     # ЭНЭ ФАЙЛЫГ ХОСТООС ГАРГА
   grep "^# public key:" /root/backups-age-key.txt
   ```

   **Хувийн түлхүүрээ алдвал бүх нөөц ашиггүй болно.** Түүнийг нууц үгийн
   менежерт эсвэл офлайн хадгал — нөөц байгаа машин дээр биш.

2. **Суулгац өөрийн түүхээ устгаж чадахгүй.** Bucket нь versioning-тэй, суулгац
   бүрийн түлхүүр нь `PutObject`, `GetObject`, `ListBucket` авдаг ба
   `DeleteObject` авдаггүй. Эвдэрсэн хост нэмж чадна, устгаж чадахгүй — энэ нь
   ransomware-ийг давж гардаг шинж чанар.

3. **Зөвхөн S3 API гадагш харна.** MinIO-гийн консол нь loopback дээр үлдэнэ
   (127.0.0.1:9003, ssh tunnel-ээр). Хоёр дахь нэвтрэх гадаргуу бөгөөд түүний
   ард бүх суулгацын өгөгдөл байна.

**Суулгац бүрт bucket ба түлхүүр өгөх** — `mc mb`, `mc version enable`,
дээрх бодлоготой `mc admin policy create`, `mc admin user add`. Дараа нь эх
хост дээр:

```bash
sudo tee /etc/default/nexus-backup >/dev/null <<'ENV'
BACKUP_AGE_RECIPIENT=age1...        # НИЙТИЙН түлхүүр
BACKUP_S3_ENDPOINT=https://backups.nexus.gerege.mn
BACKUP_S3_BUCKET=nexus-backups-<суулгац>
BACKUP_S3_KEY=backup-<суулгац>
BACKUP_S3_SECRET=...
ENV
sudo chmod 600 /etc/default/nexus-backup
sudo sh -c 'echo "15 3 * * * root . /etc/default/nexus-backup && /usr/local/bin/nexus-backup.sh >> /var/log/nexus-backup.log 2>&1" > /etc/cron.d/nexus-backup'
```

Аль нэг тохиргоо дутуу бол алхам бүхэлдээ алгасагдана — тохируулаагүй суулгац
скриптийг ажиллуулж чадах ёстой. Гэхдээ `nexus_backup_offsite_ok` нь 0 хэвээр
байх ба хоёр цагийн дараа `NexusBackupNotLeavingTheHost` дуугарна: хуулбар өөр
газар байхгүй гэдэг нь чимээгүй өнгөрөх ёсгүй баримт.

Домэйны үндэс дээр тайлбарын хуудас байна
(`deploy/nginx/backups-landing/index.html`). S3-ийн клиент хүсэлт бүрээ гарын
үсэг зурдаг, браузер зурдаггүй — vhost нь `Authorization` толгойгүй `GET /`-д
хуудсыг, бусад бүхэнд MinIO-г өгнө. Хуудсыг байрлуулах:

```bash
sudo mkdir -p /var/www/backups
sudo cp deploy/nginx/backups-landing/index.html /var/www/backups/index.html
```

### Сэргээх

Нөөцлөлтийг сэргээж үзээгүй бол тэр нь нөөцлөлт биш, зөвхөн итгэл найдвар.

```bash
# 1. сангаас татах
docker exec nexus_backups_minio mc cp store/nexus-backups-nexus/<файл>.age /tmp/
docker cp nexus_backups_minio:/tmp/<файл>.age .

# 2. хувийн түлхүүрээр тайлах
age -d -i backups-age-key.txt <файл>.age > dump.sql.gz

# 3. хаях зориулалттай санд сэргээж шалгах
docker exec gerege_nexus_postgres psql -U postgres -c 'CREATE DATABASE restore_check'
gunzip -c dump.sql.gz | docker exec -i gerege_nexus_postgres psql -U postgres -d restore_check
docker exec gerege_nexus_postgres psql -U postgres -d restore_check -c 'SELECT count(*) FROM registry.tenants'
docker exec gerege_nexus_postgres psql -U postgres -c 'DROP DATABASE restore_check'
```

Үр дүнг консолын Нөөцлөлт дэлгэц дээрээс бүртгэ — `platform_backups` дотор
`restore_test` төрлөөр хадгалагдана, ингэснээр сэргээлт хамгийн сүүлд хэзээ
шалгагдсаныг хожим асууж болно.

**Nexus-ийн өөрийн хуулбарын хувьд энэ сан нь нэг машин дээр байна.** Хүснэгт
устгасан, буруу migration, volume дахин үүсгэсэн зэргээс хамгаална — хостоо
алдахаас хамгаалахгүй. Бусад хост дээрх суулгацуудын хувьд энэ нь жинхэнэ өөр
байршил юм. Сүүлийн цоорхойг хаах алхам нь энэ санг өөр газар руу
толилуулах — `mc mirror` таймер дээр.

---

## 9. Гаднаас шалгах — Uptime Kuma

**Энэ репод deploy хийгдэхгүй, зориудаар.** Энэ хост дээр ажиллаж байгаа
монитор нь хост унахад хамт унана — "монитор өөрөө унавал хэн мэдэх вэ"
гэсэн асуултын хариулт нь өөр хаяган дээр байх ёстой.

Хямд VPS эсвэл өөр үүлэн дээр:

```bash
docker run -d --restart unless-stopped \
  -p 3001:3001 \
  -v uptime-kuma:/app/data \
  --name uptime-kuma louislam/uptime-kuma:1
```

Тохируулах шалгалтууд:

| Төрөл | Хаяг | Давтамж | Тайлбар |
| --- | --- | --- | --- |
| HTTP(s) | `https://nexus.gerege.mn/health` | 60с | `"status":"ok"` гэсэн үг агуулсан эсэхийг шалга |
| HTTP(s) | `https://nexus.gerege.mn/ready` | 60с | Өгөгдлийн сангийн хүртээмж |
| TLS хугацаа | дээрхтэй адил | — | Kuma өөрөө сертификатыг хардаг, 14 хоногийн сануулга |
| HTTP(s) | `https://nexus.gerege.mn/grafana/login` | 300с | Мониторинг өөрөө амьд эсэх |

Мэдэгдлийг **энэ хостоос гарсан** суваг руу тохируул — SMTP нь энэ серверийнх
байвал уналтын үед мэдэгдэл ч явахгүй.

---

## 10. Ослын үед

1. **Grafana → "API тойм"** — алдааны хувь, p95, аль зам.
2. Гадаад систем сэжигтэй бол **"Гадаад системүүд"**.
3. Удаашрал бол **"Инфраструктур"** → DB pool, Postgres холболт, диск.
4. Дохио ирсэн бол [`RUNBOOKS.md`](RUNBOOKS.md) доторх тухайн alert-ын
   хэсгийг нээ — alert бүрийн `runbook` annotation нь яг тэр холбоос.
5. Лог хэрэгтэй бол Grafana → Explore → Loki, §6-ийн query-үүд.

---

## 11. Trace — OpenTelemetry ба Tempo

### Асаах

Tempo нь мониторингийн стектэй хамт үргэлж асдаг ч **платформ түүн рүү юу ч
илгээхгүй** — тэр нь платформын шийдвэр.

Утга нь GitHub-ийн repository **variable** (Settings → Variables), нууц биш.
`.env`-ийг deploy бүр GitHub-ээс шинээр бичдэг тул серверт гараар нэмсэн мөр
дараагийн deploy хүртэл л амьдарна:

```bash
gh variable set OTEL_EXPORTER_OTLP_ENDPOINT -b "http://gerege_nexus_tempo:4318"
gh variable set OTEL_TRACES_SAMPLER_ARG -b "0.1"
```

...дараа нь дараагийн deploy backend-ыг шинэ утгатай эхлүүлнэ. Хувьсагч
нэмэхэд гурван газар засвар шаардлагатай — `docker-compose.prod.yml`-ийн
backend env, `deploy.yml`-ийн `env:` / `envs:` / `.env` heredoc, ба variable
өөрөө. Аль нэг нь дутвал чимээгүй ажиллахгүй.

**Асаалттай эсэхийг шалгах:**

```bash
docker exec gerege_nexus_backend printenv OTEL_EXPORTER_OTLP_ENDPOINT
docker logs gerege_nexus_backend 2>&1 | grep "tracing is"
curl -s localhost:3200/api/search?tags= | head -c 200        # Tempo-д trace ирсэн эсэх
``` Хоосон бол tracing нь **үнэхээр**
унтарсан: exporter байхгүй, batch processor байхгүй, background goroutine
байхгүй, код доторх span бүр no-op. Tempo огт ажиллуулахгүй суулгац
хэмжигдэхүйц зардал төлөхгүй.

### Sampling

Өгөгдмөл нь 10%. Бүгдийг авах нь ямар ч эзлэхүүн дээр буруу хариулт: хүсэлт
бүрийн доторх query бүр нэг span бөгөөд бүгд retention дуустал хадгалагдана.
10% нь хоцролтыг тодорхойлж, давтагдах удаан замыг барихад хангалттай — тэр
хоёр л зүйлийн төлөө trace уншдаг. **Тодорхой нэг** удаан хүсэлтийг олох
хэрэгтэй бол тэр нь логийн `request_id`-ийн ажил.

`/health`, `/ready`, `/metrics` гурав нь огт trace хийгддэггүй: Docker ба
Prometheus тэднийг 10-15 секунд тутам дууддаг тул 10% дээр ч бүх trace-ийн
дийлэнх нь тэд байх байсан.

### Гурвыг хооронд нь холбох

Grafana-д гурван datasource **хоорондоо холбогдсон**:

- Логийн мөр дэх `trace_id` → **Trace** товч (derived field);
- Trace дотроос → тухайн container-ийн лог, тухайн үеийн;
- Trace дотроос → үйлчилгээний RED хэмжүүр;
- Хоцролтын графикийн удаан цэг → түүнийг удаашруулсан trace (exemplar).

Энэ гурвалсан аялал бол зөвхөн эхнийхийг нь биш гурвууланг нь ажиллуулж
байгаагийн бүх шалтгаан.

### Юуг trace хийхгүй вэ

**Query-ийн параметр огт бичигдэхгүй** (`otelpgx`-ийн өгөгдмөл, түүнийг
өөрчилж болохгүй). Тэдгээр нь query ямар мөрийн тухай болохыг хэлдэг —
и-мэйл хаяг, регистрийн дугаар, орж ирж буй нууц үгийн хэш. Span нь Tempo-гийн
retention-ий турш хадгалагдаж, Grafana нээж чадах хүн бүрт уншигдана. SQL
текст өөрөө аюулгүй: тэнд зөвхөн `$1`, `$2` байна.

---

## 12. Алдааны бүртгэл — GlitchTip

Sentry-ийн протокол ярьдаг, түүнээс олон дахин хөнгөн self-hosted хувилбар.
**Тусдаа compose файл**: `deploy/docker-compose.glitchtip.yml`. Тусдаа
байгаа шалтгаан нь дөрвөн нэмэлт контейнер ба хоёр дахь Postgres — үүнийг
хүсэхгүй суулгац тугийн тухай уншилгүй татгалзаж чадах ёстой.

### Босгох

```bash
cd /opt/open-gerege-nexus
# .env.monitoring дотор GLITCHTIP_DB_PASSWORD ба GLITCHTIP_SECRET_KEY бөглөнө
docker compose -f deploy/docker-compose.glitchtip.yml --env-file .env.monitoring up -d

# Эхний хэрэглэгч (нээлттэй бүртгэл зориудаар хаалттай)
docker exec -it gerege_nexus_glitchtip_web ./manage.py createsuperuser
```

Дараа нь `http://localhost:8000` (SSH tunnel) дээр орж, байгууллага ба
төсөл үүсгээд DSN-ийг хуулна.

### Залгах

Backend — платформын `.env`-д:

```bash
SENTRY_DSN=http://<key>@gerege_nexus_glitchtip_web:8000/1
```

Frontend — DSN нь bundle дотор **build үед** шигтгэгддэг тул runtime
хувьсагч биш, image-ийн build argument:

```bash
docker build --build-arg NEXT_PUBLIC_SENTRY_DSN=<dsn> ...
```

Rendering server-ийн өөрийн алдааг runtime-ийн `FRONTEND_SENTRY_DSN`
хувьсагчаар өгнө (compose дотор `SENTRY_DSN` болж дамжина).

### Юу илгээгддэггүй вэ

Энэ бол PII-ийн заавал мөрдөх хил. Backend талд
`observability.scrubEvent`, frontend талд `beforeSend`:

| Хасагддаг | Яагаад |
| --- | --- |
| Query string | `/api/v1/verify/landed?ref=…` нь нэг удаагийн итгэмжлэл |
| Cookie, `Authorization` толгой | Амьд session, bearer token |
| Хүсэлтийн бие | Регистрийн дугаар, нууц үг, гарын үсэг зурах PDF |
| Хэрэглэгчийн и-мэйл, нэр, IP | Хүнийг нэрлэх шаардлагагүй |
| Session Replay (frontend) | Хараад байсан дэлгэцийн DOM-ыг бичдэг |

**Үлддэг**: tenant ID (хэдэн байгууллагад нөлөөлснийг тоолоход),
`request_id`, `trace_id`, route pattern, `User-Agent`. Толгойн жагсаалт нь
**allow-list** — ирээдүйд proxy нэмсэн толгой автоматаар гарахгүй.
`errortracking_internal_test.go` энэ бүхнийг шалгана.

---

## 13. Дараагийн үе шат

Тайлангийн модуль нь
[дизайны саналын](MONITORING_AND_REPORTING_PROPOSAL.md) 4-р үе шат.
