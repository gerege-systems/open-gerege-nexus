# Runbook — дохио бүрд юу хийх вэ

Prometheus-ийн дохио бүрд: **юу болсон**, **эхний 5 минутад юу шалгах**,
**яаж засах**, **хэзээ өргөжүүлэх**.

[Баримт бичгийн төв рүү буцах](README.md) ·
[Мониторингийн заавар](MONITORING.md)

Дүрмүүд нь `deploy/monitoring/prometheus/rules/` дотор. Дохио бүрийн
`runbook` annotation нь энэ файлын харгалзах хэсэг рүү заана.

---

## Ерөнхий зарчим

**Хоёр түвшин.** `severity: page` нь хэн нэгнийг сэрээх ёстой — хэрэглэгч
одоо хохирч байна. `severity: ticket` нь ажлын өдөр засах — засахгүй бол
хохирол болно. Ticket дээр 03:00 цагт хариу үйлдэл хийх нь alert fatigue-ийн
хамгийн богино зам бөгөөд түүний төгсгөл нь page-ийг ч үл тоох явдал.

**Эхлээд хэмжилт, дараа нь тааварлалт.** Дохио бүрийн эхний алхам нь
Grafana дээр аль хэсэг эвдэрсэнийг харах. Restart нь бараг үргэлж
шинжтомжийг арилгаад шалтгааныг нь нуудаг.

**Юу хийснээ бич.** Осол бүрийн дараа энэ файлыг засах нь дараагийн хүнд
өгөх хамгийн үнэ цэнэтэй зүйл.

**Тайван байдал.** Платформын ихэнх дохио нь өгөгдөл алдагдсан гэсэн үг биш.
Өгөгдөл алдагдах ганц бодит эрсдэл нь дискний дүүрэлт бөгөөд түүнд тусдаа
хоёр дохио бий.

---

## API

### NexusAPIDown

**Юу болсон.** Prometheus 2 минутын турш backend-ийн `/metrics`-ийг уншиж
чадсангүй. Процесс унасан, гацсан, эсвэл сүлжээ тасарсан.

Энэ үед **бусад бүх API дохио чимээгүй болно** — өгөгдөл ирэхгүй тул дүрэм
тооцоологдохгүй. Тиймээс энэ дохио дангаараа ирж байгаа нь "өөр юу ч болоогүй"
гэсэн үг биш.

**Эхний 5 минут.**

```bash
docker ps --filter name=gerege_nexus_backend
docker logs --tail 200 gerege_nexus_backend
curl -s localhost:8082/health
curl -s localhost:8082/ready
```

- Контейнер огт байхгүй эсвэл `Restarting` бол → логоос яагаад гарсныг унш.
- `/health` хариулж байгаа атлаа `/metrics` уншигдахгүй бол → сүлжээний асуудал,
  Prometheus нь platform network-д нэгдсэн эсэхийг шалга
  (`docker inspect gerege_nexus_prometheus | grep -A5 Networks`).
- `/health` OK, `/ready` 503 бол → энэ бол өгөгдлийн сангийн асуудал,
  [NexusPostgresDown](#nexuspostgresdown) руу оч.

**Засах.**

```bash
docker compose -f docker-compose.prod.yml up -d backend
```

Дахин асаахын өмнө логийг **хадгал** — контейнер дахин үүсэхэд Loki-д
очоогүй мөрүүд алга болно:

```bash
docker logs gerege_nexus_backend > /tmp/backend-crash-$(date +%s).log 2>&1
```

Байнга унаж байвал: санах ой (`docker stats`), OOM
(`dmesg | tail -50`), эсвэл миграц дуусаагүй байх магадлалтай.

**Өргөжүүлэх.** 15 минутын дотор сэргээгээгүй бол платформын хариуцсан хүнд.
Өгөгдлийн сан хөндөгдсөн сэжигтэй бол шууд.

---

### NexusAPIErrorBudgetBurningFast

**Юу болсон.** Сүүлийн 1 цаг ба сүүлийн 5 минутад хоёуланд нь 5xx-ийн хувь
1.44%-иас давлаа. Ийм хурдаар сарын алдааны төсөв 2 хоногт дуусна.

Хоёр цонх зэрэг давсан үед л дохио өгдөг гэдэг нь чухал: түр зуурын үсрэлт
энэ дохиог өгөхгүй, харин **одоо ч үргэлжилж байгаа** эвдрэл өгнө.

**Эхний 5 минут.** Grafana → **API тойм**:

1. *Алдаа хамгийн их гаргаж буй замууд* — нэг зам уу, бүгд үү?
   - Нэг зам → тэр функцийн саяхны өөрчлөлт, эсвэл түүний хамаарал.
   - Бүх зам → өгөгдлийн сан, эсвэл дөнгөж deploy хийсэн хувилбар.
2. *Хүсэлтийн урсгал* — 5xx-ийн эхэлсэн мөч deploy-тэй давхцаж байна уу?
3. Loki:
   ```logql
   {container="gerege_nexus_backend"} | json | level = "ERROR"
   ```

**Засах.**

- **Deploy-ийн дараа эхэлсэн бол → буцаа.** Оношлохоос өмнө буцаах нь зөв
  дараалал:
  ```bash
  IMAGE_TAG=<өмнөх sha> docker compose -f docker-compose.prod.yml up -d backend
  ```
- **Гадаад системээс шалтгаалсан бол** → [NexusExternalSystemFailing](#nexusexternalsystemfailing).
- **Өгөгдлийн сангаас** → [NexusDatabasePoolExhausted](#nexusdatabasepoolexhausted).

**Өргөжүүлэх.** 30 минутын дотор алдааны хувь буурахгүй бол.

---

### NexusAPIErrorBudgetBurning

**Юу болсон.** 6 цаг ба 30 минутын цонхонд 0.6%-иас дээш. Огцом уналт биш,
тогтвортой эвдрэлтэй төлөв — ихэвчлэн нэг зам, нэг гадаад систем, эсвэл
зөвхөн тодорхой нөхцөлд гардаг алдаа.

**Эхний 5 минут.** Дээрхтэй адил, гэхдээ 6 цагийн хугацаанд:
*Алдаа хамгийн их гаргаж буй замууд* панелийн хугацааг 6 цаг болго. Тогтмол
нэг зам харагдвал тэр нь шалтгаан.

**Засах.** Ихэвчлэн код засвар шаардана. Хэрэглэгчид үргэлжлүүлэн хохирч
байгаа тул засвар нь дараагийн ээлжийн ажил биш, өнөөдрийнх.

**Өргөжүүлэх.** Ажлын цагт хариуцсан багт. Шөнө сэрээх шаардлагагүй —
энэ нь `page` боловч 15 минутын `for`-той бөгөөд дэлбэрэлт биш.

---

### NexusAPIErrorBudgetSlowBurn

**Юу болсон.** 3 хоног ба 6 цагийн аль алинд нь SLO-гоос давсан. Хэн ч
сэрэх шаардлагагүй, гэхдээ энэ сарын 99.9% биелэхгүй болох замдаа явж байна.

**Эхний 5 минут.** *Үлдсэн алдааны төсөв* панелийг хар. Сөрөг бол зорилт
аль хэдийн биелэхгүй болсон.

**Засах.** Тасалдсан хэдэн зам, эсвэл нэг тогтмол алдаа гаргадаг үйлдлийг
ол. Ихэвчлэн: боловсруулагдаагүй `nil`, timeout-гүй гадаад дуудлага, эсвэл
тодорхой өгөгдөл дээр л унадаг handler.

**Өргөжүүлэх.** Шаардлагагүй. Ажлын тасалбар үүсгэ.

---

### NexusAPILatencyHigh

**Юу болсон.** Урт poll-оос бусад бүх замын p95 хариу 10 минутын турш 2
секундээс давлаа.

**Эхний 5 минут.**

1. Grafana → **Тэсвэрлэлт** → *Өгөгдлийн сангийн pool*. `pgxpool_acquired_conns`
   таазанд ойрхон, `empty_acquire` өсөж байвал → шалтгаан нь pool.
2. **Гадаад системүүд** → p95. Аль нэг систем удааширсан бол handler-ууд
   түүнийг хүлээж байна.
3. **Инфраструктур** → CPU, iowait. iowait өндөр бол диск.

**Засах.**

- Pool дүүрсэн → удаан query-г ол:
  ```sql
  SELECT pid, now() - query_start AS duration, state, left(query, 120)
    FROM pg_stat_activity
   WHERE state <> 'idle' AND now() - query_start > interval '5 seconds'
   ORDER BY duration DESC;
  ```
- Гадаад систем удаан → тухайн rail-ыг түр хаах боломжтой эсэхийг хар
  (жишээ нь eSign HSM-ийг гарын үсгийн бодлогоос).
- Диск удаан → [NexusDiskFillingUp](#nexusdiskfillingup) шалга, дүүрсэн диск
  удаашралын түгээмэл шалтгаан.

**Өргөжүүлэх.** Хэрэглэгчид "систем ажиллахгүй байна" гэж мэдэгдэж эхэлбэл
`page` шиг хандах.

---

### NexusLoadShedding

**Юу болсон.** Зэрэг үйлчлэх хүсэлтийн тааз (1000)-д хүрч, шинэ хүсэлтүүд
503-аар татгалзагдаж байна. Хамгаалалт ажиллаж байгаа хэрэг — гэхдээ
хэрэглэгч алдаа хараад байна.

**Эхний 5 минут.**

1. **Тэсвэрлэлт** → *Зэрэг үйлчилж буй хүсэлт*. Огцом үсэрсэн үү, аажим
   өссөн үү?
   - Огцом → урсгалын оргил эсвэл халдлага. **API тойм** → rps.
   - Аажим → хүсэлтүүд дуусахгүй хуримтлагдаж байна: удаан гадаад дуудлага
     эсвэл өгөгдлийн сангийн түгжээ.
2. Эх сурвалж нэг IP мөн эсэхийг nginx-ийн логоос:
   ```bash
   awk '{print $1}' /var/log/nginx/access.log | sort | uniq -c | sort -rn | head
   ```

**Засах.**

- Халдлага бол → nginx дээр хязгаарла (тэнд аль хэдийн `limit_req` бүсүүд бий).
- Бодит ачаалал бол → яагаад хүсэлтүүд удаж байгааг ол; таазыг өсгөх нь
  сүүлийн арга бөгөөд ихэвчлэн буруу арга — 1000 зэрэг хүсэлт нь 25 холболттой
  pool-той машины хувьд аль хэдийн их.
- Урт poll-ууд (eID) хуримтлагдсан бол энэ нь хэвийн: тэдгээр нь секундүүдээр
  барих зориулалттай.

**Өргөжүүлэх.** 15 минутаас удаан үргэлжилбэл.

---

## Гадаад системүүд

### NexusExternalSystemFailing

**Юу болсон.** `{{ system }}` рүү хийсэн дуудлагын тэн хагасаас олон нь 10
минутын турш амжилтгүй. Энэ нь ихэвчлэн **энэ платформын доголдол биш** —
гэхдээ хэрэглэгчид нэвтэрч эсвэл гарын үсэг зурж чадахгүй байна.

Аль систем ямар үйлчилгээг зогсоох вэ:

| Систем | Зогсох зүйл |
| --- | --- |
| `eid` | eID Mongolia-гаар нэвтрэх, баталгаажсан гарын үсэг |
| `dan` | ДАН-аар нэвтрэх |
| `xyp` | Иргэн/байгууллагын лавлагаа |
| `esign` | HSM-ийн гарын үсэг (хуучин зам) |
| `gemini` | AI туслах, орчуулга, яриа таних |
| `emailverify` | Хаяг баталгаажуулах и-мэйл |

**Эхний 5 минут.**

1. **Гадаад системүүд** dashboard → *p95 — үйлдлээр*. Бүх үйлдэл унасан уу,
   нэг нь уу?
2. Гараар шалга — жишээ нь eID:
   ```bash
   docker exec gerege_nexus_backend wget -qO- --timeout=10 https://eidmongolia.mn/ >/dev/null && echo reachable
   ```
3. Loki:
   ```logql
   {container="gerege_nexus_backend"} | json | level = "ERROR" |= "eid"
   ```
4. Итгэмжлэл дууссан эсэхийг шалга — 401/403 нь тэдний уналт биш, бидний
   тохиргооны асуудал.

**Засах.**

- **Тэдний тал бол**: тухайн үйлчилгээ үзүүлэгчид мэдэгд. Хэмжилтээ хамт өг —
  `external_request_duration_seconds` нь яг хэдэн цагт, ямар хувьтай гэдгийг
  хэлнэ.
- **Бидний тал бол** (итгэмжлэл, тохиргоо): `.env`-ийг шалга, backend-ыг
  дахин асаа.
- **Хэрэглэгчдэд**: өөр нэвтрэх зам байгаа эсэхийг мэдэгд (eID унасан ч нууц
  үг, Google ажиллаж байгаа бол).

**Өргөжүүлэх.** Нэвтрэлтийн бүх зам (eid, dan, google, sso) зэрэг унасан бол
шууд — тэр үед хэн ч орж чадахгүй байна.

---

### NexusExternalSystemDegraded

**Юу болсон.** 30 минутын турш алдааны хувь 15%-иас дээш. Зургаа тутмын нэг
хүсэлт унана — хэрэглэгчийн хувьд "заримдаа ажилладаг", энэ нь бүрэн
уналтаас **удаан анзаарагддаг** тул илүү их гомдол авчирдаг.

**Эхний 5 минут.** Дээрхтэй адил. Нэмж: алдаа тодорхой хэв маягтай юу —
тодорхой регистрийн дугаар, тодорхой хэмжээний файл?

**Засах.** Ихэвчлэн хүлээх нь зөв — эсвэл давтан оролдох логик нэмэх.
Дуудлага бүр өөрийн timeout-той эсэхийг шалга.

**Өргөжүүлэх.** Шаардлагагүй. Ажлын тасалбар.

---

### NexusExternalSystemSlow

**Юу болсон.** `{{ system }}`-ийн p95 хариу 10 секундээс удаан. Удаан гадаад
дуудлага нь хүсэлтийн goroutine болон **pool-ын холболтыг** барьж байдаг тул
тухайн систем рүү огт хандаагүй хүсэлтүүд ч удаашрана.

**Эхний 5 минут.** Pool-ын байдлыг шалга (**Тэсвэрлэлт** dashboard).
`resilience_in_flight_requests` өсөж байвал энэ нь удахгүй load shedding
болно.

**Засах.** Тухайн rail-ыг түр хаах, эсвэл timeout-ыг богиносгох. Хамгийн
муу төлөв нь удаан дуудлага + урт timeout: платформ бүхэлдээ нэг гадаад
системийн хурдаар ажиллана.

**Өргөжүүлэх.** Load shedding эхэлбэл.

---

## Инфраструктур

### NexusDiskFillingUp

**Юу болсон.** `{{ mountpoint }}` дээр сул зай 15%-иас бага. 93% давбал
`NexusDiskAlmostFull` нь `page` болж ирнэ.

**Энэ бол өгөгдөл алдагдах бодит эрсдэл бүхий цөөхөн дохионы нэг.** 100%
хүрэхэд Postgres бичихээ болино, WAL бичигдэхгүй, платформ бүхэлдээ зогсоно.

**Эхний 5 минут.**

```bash
df -h
du -xh --max-depth=1 / 2>/dev/null | sort -rh | head -20
docker system df
```

Хамгийн түгээмэл гурван шалтгаан:

1. **Docker-ийн хуучин image, build cache** — deploy бүр шинэ image татдаг.
2. **Prometheus/Loki-ийн өгөгдөл** — 20 GB / 31 хоногийн таазтай ч эхний
   тохиргооны алдаа таазыг хэтрүүлж болно.
3. **Postgres** — үнэхээр өссөн, эсвэл vacuum ажиллаагүй.

**Засах.**

```bash
# Хамгийн аюулгүй, ихэвчлэн хамгийн үр дүнтэй
docker image prune -af --filter "until=168h"
docker builder prune -af

# Prometheus-ийн эзлэх хэмжээ
docker exec gerege_nexus_prometheus du -sh /prometheus

# Postgres-ийн хамгийн том хүснэгтүүд
docker exec -i gerege_nexus_postgres psql -U postgres -d platform_db -c "
  SELECT relname, pg_size_pretty(pg_total_relation_size(c.oid)) AS size
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
   WHERE n.nspname = 'public' AND c.relkind = 'r'
   ORDER BY pg_total_relation_size(c.oid) DESC LIMIT 15;"
```

**Устгаж болохгүй зүйлс:** `gerege_nexus_postgres_data` volume, шифрлэлтийн
түлхүүр агуулсан `.env` файлууд, гарын үсэгтэй PDF-үүд.

**Өргөжүүлэх.** 95% давбал шууд — цаг хэдхэн байна.

---

### NexusDiskAlmostFull

93%. Дээрхтэй ижил, гэхдээ `page`. Эхлээд `docker image prune -af` —
ихэвчлэн хэдэн арван GB чөлөөлдөг бөгөөд аюулгүй.

---

### NexusHostMemoryPressure

**Юу болсон.** Сул санах ой 10%-иас бага. Дараагийн алхам нь OOM killer
бөгөөд тэр нь ихэвчлэн хамгийн том процессыг — Postgres-ийг — алдаг.

**Эхний 5 минут.**

```bash
free -h
docker stats --no-stream
dmesg | grep -i -E 'oom|killed process' | tail -20
```

**Инфраструктур** dashboard → *Контейнер бүрийн санах ой*. Аль контейнер
өссөн бэ, тогтмол өсөж байна уу (leak) эсвэл нэг үсрэлт үү?

**Засах.**

- Мониторингийн стек өөрөө хэт их идэж байвал → Prometheus-ийн retention
  багасга, эсвэл Loki-г түр унтраа. Платформ энэ хоёрын алийг нь ч
  шаарддаггүй.
- Backend leak → дахин асаа, дараа нь `go_goroutines` графикийг хар
  (**API тойм**). Тогтмол өсөж байгаа goroutine нь дуусаагүй урт poll.
- Postgres → `shared_buffers` хэт өндөр тохируулсан эсэхийг шалга.

**Өргөжүүлэх.** OOM killer аль хэдийн ажилласан бол шууд.

---

### NexusPostgresDown

**Юу болсон.** postgres_exporter өгөгдлийн санд холбогдож чадахгүй байна.

**Эхлээд ялга:** үнэхээр Postgres унасан уу, эсвэл зөвхөн **exporter-ийн
нэвтрэлт** эвдэрсэн үү? Хоёр дахь нь платформд огт нөлөөгүй.

```bash
# Платформ өөрөө өгөгдлийн санд хүрч байна уу
curl -s localhost:8082/ready

docker ps --filter name=gerege_nexus_postgres
docker logs --tail 100 gerege_nexus_postgres
```

`/ready` нь `{"status":"ready"}` гэж хариулж байвал → Postgres хэвийн,
асуудал нь exporter-ийн нууц үг. `MONITORING.md` §2.2-ыг дахин хий.

**Postgres үнэхээр унасан бол.**

```bash
docker compose -f docker-compose.prod.yml up -d postgres
docker logs -f gerege_nexus_postgres
```

Диск дүүрсэн эсэхийг **заавал** шалга — Postgres-ийн зогсох хамгийн түгээмэл
шалтгаан бол сул зай дуусах.

**Өргөжүүлэх.** Postgres үнэхээр унасан бол шууд. Өгөгдөл эвдэрсэн шинж
тэмдэг (`PANIC`, `corrupted`) харагдвал **дахин асаахаа боль** — эхлээд
backup-аа шалга.

---

### NexusPostgresConnectionsHigh

**Юу болсон.** Зөвшөөрөгдсөн холболтын 80% эзлэгдсэн. Дүүрэхэд шинэ холболт
бүр татгалзагдана.

Платформын pool нь replica тутамд 25-аар хязгаарлагдсан тул энэ нь ихэвчлэн
**pool-оос гадуурх** ямар нэг зүйл холбогдож байгааг илтгэнэ.

**Эхний 5 минут.**

```sql
SELECT usename, application_name, state, count(*)
  FROM pg_stat_activity GROUP BY 1,2,3 ORDER BY 4 DESC;
```

- `idle in transaction` олон → commit хийгээгүй гүйлгээ. Түгжээ барьж,
  vacuum-ыг зогсооно.
- Танихгүй `application_name` → өөр стек, нээлттэй үлдсэн `psql`, эсвэл
  дуусаагүй миграц.

**Засах.**

```sql
-- 10 минутаас удаан idle in transaction байгаа холболтуудыг таслах
SELECT pg_terminate_backend(pid) FROM pg_stat_activity
 WHERE state = 'idle in transaction'
   AND now() - state_change > interval '10 minutes';
```

`max_connections`-ыг өсгөх нь сүүлийн арга: холболт бүр санах ой иддэг,
өсгөх нь ихэвчлэн асуудлыг хойшлуулна.

**Өргөжүүлэх.** 95% давбал.

---

### NexusDatabasePoolExhausted

**Юу болсон.** Платформын өөрийн pool-ын 90%-иас дээш нь 10 минутын турш
эзлэгдсэн. Хүсэлтүүд холболт хүлээж эхэлнэ — **энэ нь удаашралыг тайлбарлах
хамгийн эхний газар**.

Postgres чөлөөтэй атлаа энэ дүүрсэн байх нь бүрэн боломжтой: асуудал нь
холболтын тоо биш, тэднийг барьж буй хугацаа.

**Эхний 5 минут.**

1. **Тэсвэрлэлт** → *Хүлээлт*. `empty_acquire` тэгээс салсан мөч нь эхлэл.
2. Удаан query:
   ```sql
   SELECT pid, now() - query_start AS duration, left(query, 200)
     FROM pg_stat_activity
    WHERE state = 'active' AND now() - query_start > interval '2 seconds'
    ORDER BY duration DESC;
   ```
3. Түгжээ:
   ```sql
   SELECT blocked.pid AS blocked_pid, blocking.pid AS blocking_pid,
          left(blocked.query, 80) AS blocked_query
     FROM pg_stat_activity blocked
     JOIN pg_stat_activity blocking
       ON blocking.pid = ANY(pg_blocking_pids(blocked.pid));
   ```

**Засах.** Удаан query-г ол, индекс нэм. Түр арга нь тухайн query-г таслах
(`pg_cancel_backend`). Pool-ын хэмжээг өсгөх нь Postgres руу асуудлыг
шилжүүлнэ.

**Өргөжүүлэх.** p95 2 секундээс давбал.

---

### NexusRedisDown

**Юу болсон.** Redis хариу өгөхгүй байна.

**Платформ ажиллана.** Redis нь энд кэш хүчингүй болгох мессеж болон
deployment-ийн хэмжээний хурдны хязгаарын тоолуурыг барьдаг. Унасан үед
хязгаар нь процесс тус бүрийнх болж, эрх цуцлах нь бусад replica дээр
кэшийн TTL-ийн хугацаагаар хоцрогдоно. Өгөгдөл алдагдахгүй — Redis-д
хадгалагдах ёстой юу ч байхгүй.

**Эхний 5 минут.**

```bash
docker ps --filter name=gerege_nexus_redis
docker logs --tail 50 gerege_nexus_redis
docker exec gerege_nexus_redis redis-cli ping
```

**Засах.**

```bash
docker compose -f docker-compose.prod.yml up -d redis
```

Санах ойн таазанд хүрсэн бол энэ нь хэвийн — `allkeys-lru` бодлого хуучин
түлхүүрүүдийг хаяна.

**Өргөжүүлэх.** Шаардлагагүй.

---

### NexusContainerRestarting

**Юу болсон.** `{{ name }}` сүүлийн 30 минутад 3-аас олон удаа дахин эхэллээ.

Docker-ийн `restart: unless-stopped` нь эвдрэлийг нуух гол арга: үйлчилгээ
"ажиллаж байгаа" харагдана, гэхдээ хэсэг хугацаанд хариу өгөхгүй бөгөөд лог
нь контейнер бүрийн хамт алга болно.

**Эхний 5 минут.**

```bash
docker inspect gerege_nexus_<нэр> --format '{{.State.ExitCode}} {{.State.Error}}'
docker logs --tail 300 gerege_nexus_<нэр>
```

- ExitCode 137 → OOM. [NexusHostMemoryPressure](#nexushostmemorypressure).
- ExitCode 1 + тохиргооны алдаа → `.env`.
- Healthcheck унаж байгаа → шалтгаан нь тэр контейнер биш, түүний хамаарал.

**Засах.** Шалтгааныг зас. Дахин асаах нь мөчлөгийг л үргэлжлүүлнэ.

**Өргөжүүлэх.** `gerege_nexus_backend` эсвэл `gerege_nexus_postgres` бол шууд.

---

### NexusTLSCertificateExpiringSoon

**Юу болсон.** `{{ domain }}`-ийн сертификат 14 хоногт дуусна — өөрөөр хэлбэл
certbot-ын автомат сунгалт ажиллаагүй байна.

Дуусахад бүх браузер сэрэмжлүүлэг үзүүлнэ, **native клиентүүд огт
холбогдохоо болино** (тэд сертификатын алдааг алгасах товчгүй).

**Эхний 5 минут.**

```bash
sudo certbot certificates
sudo systemctl status certbot.timer
sudo journalctl -u certbot --since '7 days ago' | tail -50
```

**Засах.**

```bash
sudo certbot renew --dry-run   # эхлээд туршилт
sudo certbot renew
sudo nginx -t && sudo systemctl reload nginx
```

Түгээмэл шалтгаан: `.well-known/acme-challenge/` руу очих зам nginx-д
хаагдсан, DNS өөрчлөгдсөн, эсвэл 80 порт хаалттай.

**Өргөжүүлэх.** 3 хоног үлдсэн бол шууд.

---

### NexusTLSExpiryUnknown

**Юу болсон.** node_exporter-ийн textfile collector-т сертификатын хугацаа
бичигдэхээ больсон.

Энэ нь өөрөө асуудал биш — **сертификат дуусахыг хэн ч мэдэхгүй** болсон
гэсэн үг. "Хэмжилт байхгүй" нь dashboard дээр "бүх зүйл хэвийн"-тэй яг
адилхан харагддаг.

**Засах.** [`MONITORING.md` §8](MONITORING.md#8-tls-ийн-хугацаа) доторх cron
ажлыг дахин суулга, эсвэл гараар ажиллуулж алдааг нь хар:

```bash
sudo /usr/local/bin/nexus-tls-expiry.sh
cat /var/lib/node_exporter/nexus_tls.prom
```

**Өргөжүүлэх.** Шаардлагагүй.

---

## Мониторинг өөрөө

### NexusMonitoringTargetDown

**Юу болсон.** `{{ job }}` scrape target 10 минутын турш хариу өгсөнгүй.
Тухайн exporter-ийн хэмжүүрүүд dashboard дээр хоосон болно.

**Эхний 5 минут.**

```bash
curl -s localhost:9091/api/v1/targets \
  | jq '.data.activeTargets[] | select(.health!="up") | {job:.labels.job, lastError}'
docker ps --filter name=gerege_nexus_
```

**Засах.**

```bash
docker compose -f deploy/docker-compose.monitoring.yml \
  --env-file .env.monitoring up -d <service>
```

`job="nexus_api"` бол энэ нь мониторингийн асуудал биш —
[NexusAPIDown](#nexusapidown).

**Өргөжүүлэх.** Шаардлагагүй, `nexus_api` биш бол.

---

### NexusAlertmanagerNotNotifying

**Юу болсон.** Мэдэгдэл илгээх оролдлого амжилтгүй болж байна. **Бүх alert
ажиллаж байгаа ч хэн ч хүлээж авахгүй байна** — өөрөөр хэлбэл энэ дохио
өөрөө ч хүрэхгүй байх магадлалтай (Grafana дээр л харагдана).

**Эхний 5 минут.**

```bash
docker logs --tail 100 gerege_nexus_alertmanager
docker logs gerege_nexus_alertmanager 2>&1 | grep 'notification channels'
```

Түгээмэл шалтгаан: SMTP нууц үг өөрчлөгдсөн, Telegram bot группээс хасагдсан,
гарах 587/465 порт хаалттай.

**Засах.** `.env.monitoring`-ийг зас, дараа нь:

```bash
docker compose -f deploy/docker-compose.monitoring.yml \
  --env-file .env.monitoring up -d alertmanager
```

Туршилтын дохио илгээх:

```bash
curl -s -XPOST localhost:9093/api/v2/alerts -H 'Content-Type: application/json' -d '[{
  "labels": {"alertname":"TestAlert","severity":"page","service":"api"},
  "annotations": {"summary":"Мэдэгдлийн суваг шалгаж байна"}}]'
```

**Өргөжүүлэх.** Шаардлагагүй, гэхдээ **энэ дохиог үл тоох нь бусад бүх
дохиог үл тоохтой тэнцэнэ**.

---

### NexusPrometheusRuleEvaluationFailing

**Юу болсон.** Дүрмийн илэрхийлэл унаж байна. Унасан дүрэм хэзээ ч дохио
өгөхгүй.

**Эхний 5 минут.**

```bash
docker logs --tail 100 gerege_nexus_prometheus | grep -i 'rule'
curl -s localhost:9091/api/v1/rules | jq '.data.groups[].rules[] | select(.health!="ok")'
```

Ихэвчлэн: хэмжүүрийн нэр өөрчлөгдсөн (код талын refactor), эсвэл дүрэм
засварласны алдаа.

**Засах.** `deploy/monitoring/prometheus/rules/`-ийг зас, дараа нь reload —
restart шаардлагагүй:

```bash
curl -X POST localhost:9091/-/reload
```

**Өргөжүүлэх.** Шаардлагагүй.

---

### NexusLokiRejectingLogs

**Юу болсон.** Loki 30 минутын турш ямар ч лог хүлээж аваагүй. Alloy
ажиллахаа больсон, эсвэл Loki бичилтийг татгалзаж байна.

Лог зөвхөн `docker logs` дотор үлдэж байна — **контейнер дахин үүсэхэд алга
болно**, өөрөөр хэлбэл дараагийн осол мөрдөх мөргүй өнгөрнө.

**Эхний 5 минут.**

```bash
docker logs --tail 100 gerege_nexus_alloy
docker logs --tail 100 gerege_nexus_loki
curl -s localhost:12345/metrics | grep loki_write
```

- Alloy-д `permission denied` → Docker socket-ийн эрх.
- Loki-д `rate limit exceeded` → `ingestion_rate_mb` хэтэрсэн: ямар нэг
  контейнер лог үер үүсгэж байна.
- Loki-д `no space left` → [NexusDiskFillingUp](#nexusdiskfillingup).

**Засах.**

```bash
docker compose -f deploy/docker-compose.monitoring.yml \
  --env-file .env.monitoring up -d alloy loki
```

**Өргөжүүлэх.** Шаардлагагүй. Гэхдээ энэ байдалтай ослыг мөрдөх нь хэд
дахин хэцүү болно гэдгийг санаж, эхэлж зас.

---

## Нөөцлөлт

### NexusBackupNeverSeen

`nexus_backup_last_run_timestamp_seconds` цуврал огт байхгүй. Нөөцлөлт унасан
гэсэн үг **биш** — нөөцлөлт байгаа эсэхийг хэн ч хэмжихгүй байна гэсэн үг, тэр
нь илүү муу.

1. Скрипт хостод байгаа эсэх: `ls -l /usr/local/bin/nexus-backup.sh`
2. Cron бичигдсэн эсэх: `cat /etc/cron.d/nexus-backup`
3. textfile хавтас байгаа эсэх: `ls -ld /var/lib/node_exporter`
4. Байхгүй бол `docs/MONITORING.md` §8а-гийн блокийг буулгаад ажиллуул.

Хамгийн сүүлд гарсан: 2026-08-30, nexus.gerege.mn — гурвуулаа байхгүй байсан.

### NexusBackupStale

Скрипт ажиллаж байгаа ч 26 цагийн турш амжилттай болоогүй.

1. `tail -50 /var/log/nexus-backup.log`
2. Дискний зай: `df -h /` — dump нь шахагдсандаа хэдэн зуун KB, гэхдээ хост
   дүүрсэн бол бичиж чадахгүй.
3. Postgres контейнерийн нэр өөрчлөгдсөн эсэх: скрипт `POSTGRES_CONTAINER`-ыг
   `gerege_nexus_postgres` гэж таамаглана.
4. Гараар ажиллуулж алдааг нь хар: `sudo /usr/local/bin/nexus-backup.sh`

### NexusBackupFailing

Сүүлийн ажиллагаа бүтээгүй. Дээрхтэй ижил алхмууд. Скрипт нь 10 KB-аас жижиг
гаралтыг зориудаар татгалздаг: алдаагүй дуусаад юу ч бичээгүй pg_dump нь
амжилттай харагдах бүтэлгүйтлийн сонгодог хэлбэр.

## Операторын консол

Консолын дохионууд бусдаасаа өөр асуулт тавьдаг: "систем эрүүл үү" биш,
"хэн нэгэн орох гэж оролдож байна уу". Консол нь nginx-ийн хаягийн
жагсаалтын ард байдаг тул эдгээр оролдлого нь **зөвшөөрөгдсөн сүлжээнээс**
ирж байна — гаднаас ирсэн бол энэ хүртэл хүрэхгүй байсан.

### NexusControlPlaneLoginFailures

**Юу болсон.** 15 минутын дотор консол руу 5-аас олон удаа амжилтгүй нэвтрэх
оролдлого болов.

**Эхний 5 минут.**

1. Шалтгааныг ялга — Prometheus дээр:
   ```
   sum by (result) (increase(cp_login_attempts_total[1h]))
   ```
   - `bad_password` давамгайлж байвал → нууц үг тааж байна.
   - `bad_code` давамгайлж байвал → нууц үг нь **зөв** байж болзошгүй, зөвхөн
     хоёр дахь хүчин зүйл нь зогсоож байна. Энэ нь илүү яаралтай.
   - `unknown` давамгайлж байвал → байхгүй и-мэйл хаягуудыг оролдож байна:
     жагсаалт таамаглаж байгаа хэрэг.
   - `no_second_factor` → bootstrap тасалдсан бүртгэл байна. §3.4-ийг үз.
2. Аль хаягнаас ирснийг:
   ```bash
   grep '/api/platform/v1/session' /var/log/nginx/access.log | awk '{print $1}' | sort | uniq -c | sort -rn | head
   ```
3. Тэр хаяг хэний вэ? `cp-allowlist.conf` дотор хэн байгааг хар.

**Засах.**

- Оператор өөрөө андуурсан бол → түгжээ 15 минутын дараа өөрөө тайлагдана.
  Яаралтай бол DB-ээс:
  ```sql
  UPDATE operator_accounts SET failed_login_attempts = 0, locked_until = NULL
   WHERE lower(email) = lower('...');
  ```
- Танихгүй хаяг бол → тэр хаягийг `cp-allowlist.conf`-оос хас, nginx reload.
  Дараа нь тухайн сүлжээнд юу болж байгааг шалга: жагсаалтад байгаа хаягнаас
  оролдлого ирсэн нь тэр машин эсвэл VPN-ийн бүртгэл алдагдсан гэсэн үг.
- `bad_password` олон боловч дараа нь `success` гарсан бол → тэр session-ыг
  `operator_audit`-аас шалга: хэн, хаанаас, юу хийсэн.

**Өргөжүүлэх.** `bad_code` олноор гарсан, эсвэл амжилтгүйн дараа амжилттай
нэвтрэлт болсон бол — тухайн операторт **утсаар** холбогдож, өөрөө мөн эсэхийг
асуу. Тийм биш бол бүх session-ыг цуцалж, нууц үг ба TOTP-ийг дахин үүсгэ.

### NexusControlPlaneLockout

**Юу болсон.** Аль хэдийн түгжигдсэн бүртгэл рүү оролдлого үргэлжилж байна.
Хүн андуурсан бол зогсдог; үргэлжилж байгаа нь автоматжуулсан оролдлого.

**Эхний 5 минут.**

1. Дээрхтэй ижил — аль бүртгэл, аль хаягнаас.
2. `operator_audit`-д тэр бүртгэлээр амжилттай юу болсныг шалга:
   ```sql
   SELECT created_at, action, ip FROM operator_audit
    WHERE operator_email = '...' ORDER BY created_at DESC LIMIT 50;
   ```

**Засах.**

1. Хаягийг `cp-allowlist.conf`-оос **шууд хас**, `nginx -t && systemctl reload nginx`.
2. Бүртгэлийг идэвхгүй болго:
   ```sql
   UPDATE operator_accounts SET disabled_at = NOW() WHERE lower(email) = lower('...');
   UPDATE operator_sessions SET revoked_at = NOW()
    WHERE operator_id = (SELECT id FROM operator_accounts WHERE lower(email) = lower('...'))
      AND revoked_at IS NULL;
   ```
3. Бусад операторуудад мэдэгд.

**Өргөжүүлэх.** Шууд. Энэ бол хамгийн эрхтэй интерфэйс рүү чиглэсэн
үргэлжилсэн оролдлого бөгөөд аль хэдийн дотоод сүлжээнд байгаа хэн нэгнээс
ирж байна.

### NexusBreakGlassUsed

**Юу болсон.** Онцгой байдлын (break-glass) операторын бүртгэлээр консол руу
нэвтэрлээ. Энэ бүртгэлийн нууц үг офлайн сейфэнд байдаг бөгөөд ердийн үед
хэзээ ч хэрэглэгддэггүй.

**Эхний 5 минут.**

1. **Хэн нээв?** Сейфийг хариуцагчаас утсаар асуу — мессежээр биш.
2. Хаанаас:
   ```bash
   docker logs gerege_nexus_api 2>&1 | grep "BREAK GLASS"
   ```
   Мөр нь и-мэйл ба IP-г агуулна.
3. Тэр session юу хийснийг:
   ```sql
   SELECT created_at, action, target_type, target_id, reason, ip
     FROM operator_audit
    WHERE operator_email = '<break-glass email>'
    ORDER BY created_at DESC LIMIT 100;
   ```

**Засах.**

- **Төлөвлөгөөт бол** (бүх superadmin-ий TOTP алдагдсан г.м.): ажил дууссаны
  дараа нууц үгийг нь ЗААВАЛ солиж, сейфэнд шинээр хийнэ. Дараа нь энгийн
  superadmin бүртгэлүүдийг сэргээ.
- **Төлөвлөгөөт биш бол** — зөвшөөрөлгүй хандалт гэж үз:
  ```sql
  UPDATE operator_accounts SET disabled_at = NOW() WHERE break_glass;
  UPDATE operator_sessions SET revoked_at = NOW() WHERE revoked_at IS NULL;
  ```
  Дараа нь `cp-allowlist.conf`-оос танихгүй хаягийг хас, nginx-ийг reload
  хийж, `operator_audit`-ыг бүхэлд нь шалга.

**Өргөжүүлэх.** Шууд, хэн нэгнийг сэрээж. Энэ бол платформын хамгийн эрхтэй
хаалга бөгөөд дуут дохио нь зориудаар чанга.

### NexusControlPlaneUnrecordedWrite

**Юу болсон.** Консолын хүсэлт 500 буцаалаа. Хамгийн магадлалтай шалтгаан нь
`RequireAudit` — `Service.Do`-г тойрч бичсэн handler-ийн хариуг барьсан.

**Эхний 5 минут.**

```bash
docker logs gerege_nexus_api 2>&1 | grep -i "without an audit record"
docker logs gerege_nexus_api 2>&1 | grep -i "control plane:" | tail -30
```

- "answered successfully without an audit record" → кодын алдаа. Тухайн
  handler `Do`-г ашиглах ёстой. Мөрөнд бичигдсэн `path` нь хаанаас хайхыг
  хэлнэ.
- "is not available (run the migrations up to 00049_control_plane)" → миграц
  ажиллаагүй. `docker compose ... run --rm migrate up`.
- "permission denied for table ..." → operator role-д тухайн хүснэгтийн эрх
  олгогдоогүй. Энэ нь **зөв ажиллаж буй** хамгаалалт: query нь тэр хүснэгтийг
  унших ёстой эсэхийг эхлээд шийд, дараа нь миграцаар тодорхой нэмнэ.

**Засах.** Кодын алдаа тул засвар нь commit. Түр зуурын тойрох арга байхгүй —
audit-гүй бичилтийг зөвшөөрөх нь энэ давхаргын утгыг бүхэлд нь алдагдуулна.

**Өргөжүүлэх.** Ажлын өдөр. Гэхдээ өгөгдлийн санд ул мөргүй өөрчлөлт орсон
байж болзошгүй тул тухайн үйлдлийг гараар шалга.
