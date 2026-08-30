# Тайлан — reporting модуль

`io.gerege.nexus.reports`: суулгасан апп бүрийн тайланг нэг дэлгэцээс
ажиллуулах, график харах, Excel/CSV болгон гаргах, товлосон хугацаанд илгээх.

Тайлангийн engine нь цөмийн `internal/workspace/reporting`-д, reports UI module нь
`client-gerege-nexus` distribution-д байна. Гаднын module engine-ийн internal
package-ийг импортлохгүй; тайлангаа нийтийн `pkg/nexus` SDK-аар бүртгэнэ.

[Баримт бичгийн төв рүү буцах](README.md) ·
[Мониторинг](MONITORING.md) ·
[Модуль бичих заавар](MODULE_AUTHORING_GUIDE.md)

---

## 1. Гол санаа: тайлан бол дэлгэц биш, тунхаглал

Модуль өөрийн тайлангаа Go-гийн `Report` интерфейсээр **тунхаглана**: юу гэж
нэрлэгдэхээ (7 хэлээр), ямар үзүүлэлт хүлээж авахаа, ямар багана гаргахаа,
хэрхэн тооцоолохоо. Бусад бүхэн — жагсаалтын дэлгэц, үзүүлэлтийн форм,
хүснэгт, график, Excel гаргалт, хуваарь, audit бүртгэл — **нэг удаа** энэ
давхаргад бичигдсэн бөгөөд аль ч модулийн аль ч тайланд үйлчилнэ.

Энэ бол Odoo-гийн загвар бөгөөд `reports` модуль billing, inventory, esign
гэсэн үг мэдэхгүй байгаагийн шалтгаан. Эсрэгээр нь хийвэл тайлангийн модуль
бусад бүх модулийг import хийх ба тэр нь энэ архитектурын зайлсхийхийг зорьсон
холбоо юм.

---

## 2. Шинэ тайлан нэмэх

Модулийнхаа хавтсанд `reports.go` үүсгэ:

```go
package billing

import (
    "context"
    "time"

    "github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

type revenueByMonth struct{}

func (revenueByMonth) Key() string { return "billing.revenue_by_month" }
func (revenueByMonth) App() string { return "io.gerege.nexus.billing" }

func (revenueByMonth) Titles() map[string]string {
    return map[string]string{"mn": "Орлого сараар", "en": "Revenue by month"}
}

func (revenueByMonth) Params() []nexus.ParamSpec {
    return []nexus.ParamSpec{{
        Key:           "period",
        Kind:          nexus.ParamDateRange,
        Titles:        map[string]string{"mn": "Хугацаа", "en": "Period"},
        DefaultWindow: 365 * 24 * time.Hour,
    }}
}

func (revenueByMonth) Columns() []nexus.ColumnSpec {
    return []nexus.ColumnSpec{
        {Key: "month", Kind: nexus.ColumnMonth, Chart: nexus.ChartCategory,
         Titles: map[string]string{"mn": "Сар", "en": "Month"}},
        {Key: "gross", Kind: nexus.ColumnMoney, Chart: nexus.ChartValue, Total: true,
         Titles: map[string]string{"mn": "Нийт дүн", "en": "Gross"}},
    }
}

func (revenueByMonth) Run(ctx context.Context, q nexus.Querier,
    p nexus.Params) (nexus.Result, error) {

    rows, err := q.Query(ctx, `
        SELECT date_trunc('month', created_at)::date, sum(amount + vat_amount)
          FROM tenant.billing_invoices
         WHERE tenant_id = $1 AND created_at >= $2 AND created_at <= $3
         GROUP BY 1 ORDER BY 1`,
        nexus.WorkspaceOf(ctx), p.Time("period_from"), p.Time("period_to"))
    if err != nil {
        return nexus.Result{}, err
    }
    collected, err := nexus.Collect(rows, func() (map[string]any, error) {
        var month time.Time
        var gross float64
        if err := rows.Scan(&month, &gross); err != nil {
            return nil, err
        }
        return map[string]any{"month": month, "gross": gross}, nil
    })
    if err != nil {
        return nexus.Result{}, err
    }
    return nexus.Result{Rows: collected}, nil
}
```

Модулийнхаа `New`-д бүртгэ:

```go
func New(p nexus.Platform) *BillingModule {
    m := &BillingModule{p: p}
    nexus.Register(m)
    nexus.RegisterReport(revenueByMonth{})
    return m
}
```

Дууслаа. Дэлгэц дээр гарч ирнэ, экспортлогдоно, товлогдоно, audit-д бичигдэнэ.
Frontend-д ямар ч өөрчлөлт хэрэггүй.

### Мөрдөх дүрмүүд

| Дүрэм | Яагаад |
| --- | --- |
| `WHERE tenant_id = $1` **заавал** бич | Хэрэглээний давхаргын шүүлт нь үндсэн хамгаалалт. RLS бол доод давхарга — мартсан заалтыг хоосон үр дүн болгож барих сүүлчийн тор, эхнийх нь биш |
| Мужийг `nexus.WorkspaceOf(ctx)`-оос ав | Нэгдсэн тайланд яг энэ л зүйл өөр муж болж солигдоно (§5) |
| Нэгтгэлийг SQL дотор хий | Мянган мөрийг Go руу татаад давталтаар нэмэх нь демо тенант дээр адилхан ажиллаж, бодит дээр унана |
| Хүний нэр биш, регистрийн дугаар биш | Тайлан бол экспортлогдож, и-мэйлээр явж, татсан хавтсанд үлддэг зүйл |
| `mn` гарчиг заавал | Байхгүй бол Register нь асах үед panic хийнэ |

Түлхүүр (`Key`) нь **тогтвортой**: түүгээр хуваарийн мөр, grant-ын мөр
холбогдоно. Нэрлээд өөр зүйлд дахин ашиглаж болохгүй.

---

## 3. Хамгаалалт

**Апп gate.** Тухайн аппыг суулгаагүй байгууллага түүний тайланг жагсаалтад
харахгүй, метадатаг нь авахгүй, түлхүүрээр нь дуудсан ч 404 авна. Гурвуулаа
шалгагдана — жагсаалт шүүх нь хангалттай биш, API нь тусдаа зам.

**Тенант тусгаарлалт.** Тайлан бүр дуудагчийн тенант binding дотор ажиллана
(`dbguard`, миграц 00029). Тенантын заалтаа мартсан тайлан **юу ч буцаахгүй** —
энэ нь `engine_integration_test.go`-д бодит өгөгдлийн сан дээр шалгагдсан тест.

**Зөвхөн уншина.** Тайлангийн query нь read-only гүйлгээнд ажиллана.
Бичих оролдлого нь өгөгдлийн сангаас татгалзагдана, review-ээс биш.

**30 секундын тааз.** `SET LOCAL statement_timeout` — контекстийн deadline биш
(тэр нь зөвхөн энэ процессын хүлээлтийг зогсооно). Удаан тайлан pool-ын
холболтыг барих нь тайлан удаан байснаас илүү аюултай.

**Эрх.** `reports.view` — ажиллуулах, экспортлох. `reports.schedule` —
хуваарь үүсгэх. Хоёрыг тусгаарласан шалтгаан: хуваарь бол хэн ч байхгүй үед
байгууллагын тоог хаягийн жагсаалт руу илгээх шийдвэр.

**Audit.** Гүйлт бүр (`reports.run`), экспорт бүр (`reports.export`), хуваарийн
үйлдэл бүр `audit_events`-д бичигдэнэ. Экспорт нь гүйлтээс тусдаа бичлэг:
экспорт бол өгөгдөл платформоос гарч байгаа хэрэг.

---

## 4. Товлосон тайлан

`report_schedules` хүснэгт (миграц 00045), backend доторх минут тутмын
goroutine. **Шинэ процесс байхгүй** — энэ платформ нэг бинари.

Хуваарь нь cron-ийн 5 талбар: `минут цаг өдөр сар гараг`. `0 9 1 * *` нь
сарын 1-нд 09:00. Илэрхийллийг хадгалах үед шалгана — хэзээ ч ажиллахгүй
хуваарь чимээгүй суух ёсгүй.

**Давхар илгээлт.** Хэд хэдэн replica зэрэг ажиллаж болно. Sweep нь
PostgreSQL-ийн advisory lock барьж, болзсон мөрүүдийг эхлээд `last_run_at`-аар
тэмдэглэж, дараа нь ажиллуулна. Тэмдэглээд ажиллуулах дараалал санаатай:
амжилттай илгээснийхээ дараа тэмдэглэдэг байсан бол хоёрын хооронд дахин
эхэлсэн replica тайланг хоёр удаа илгээх байсан бөгөөд хоёр дахь хувь нь
жинхэнэ хувиас ялгагдахгүй, тоо нь өөр байж болно.

### И-мэйл

```bash
REPORT_SMTP_URL=smtp://user:password@relay.example.mn:587
REPORT_MAIL_FROM=nexus@gerege.mn
```

Хоосон бол хуваарь **ажиллах боловч илгээгдэхгүй** — үр дүн нь "delivery not
configured" гэж бүртгэгдэж, дэлгэц дээр анхааруулга гарна. "Ажиллаагүй"
гэдгээс "бэлтгэгдсэн, хүргэх газаргүй" гэдэг нь өөр бөгөөд илүү хэрэгтэй
байдал.

> **Дизайнаас зөрсөн зүйл.** Санал нь товлосон тайланг "одоогийн hosted email
> үйлчилгээгээр" илгээхээр бичсэн. Тэр үйлчилгээ ганц зүйл илгээдэг —
> баталгаажуулах холбоос — бөгөөд гарчиг, бие, хавсралт өгөх endpoint байхгүй.
> Тиймээс товлосон тайланд өөрийн gate хэрэгтэй болсон ба SMTP нь суулгац
> бүрийн аль хэдийн хариулттай зүйл.

---

## 5. Тенант дамнасан тайлан

Уурхай/тээврийн компанийн кейс — саналын §3.5 — нь `report_grants` механизмаар
шийдэгдэнэ. Дэлгэрэнгүйг [`REPORT_SHARING.md`](REPORT_SHARING.md)-ээс үзнэ үү.

Энд чухал нь: тэр механизм **энэ хөдөлгүүрийг өөрчлөхгүй**. Нэгдсэн тайлан нь
ижил `Run`-ыг grantor бүрийн тенант контекст дотор дуудна, ямар ч бодлого
сулрахгүй, тайлан өөрөө ялгааг мэдэхгүй.

---

## 6. API

| Аргачлал | Зам | Тайлбар |
| --- | --- | --- |
| `GET` | `/api/v1/reports` | Аппаар бүлэглэсэн жагсаалт |
| `GET` | `/api/v1/reports/{key}` | Метадата: үзүүлэлт, багана |
| `POST` | `/api/v1/reports/{key}/run` | JSON үр дүн |
| `POST` | `/api/v1/reports/{key}/export?format=xlsx\|csv` | Файл |
| `GET` | `/api/v1/reports/schedules` | Хуваариуд |
| `POST` | `/api/v1/reports/schedules` | Хуваарь үүсгэх |
| `PUT` | `/api/v1/reports/schedules/{id}` | Засах |
| `DELETE` | `/api/v1/reports/schedules/{id}` | Устгах |

Бүгд апп gate-ийн ард. Нээлттэй тайлангийн endpoint байхгүй бөгөөд байх ч
ёсгүй.

---

## 7. Экспорт

**xlsx** (`excelize`): гарчгийн мөр, тод толгой, багана бүрийн тоо/огнооны
формат, нийт дүнгийн мөр, толгойн мөр царцаасан. Тоонууд нь **тоо** байдлаар
орно — нийлбэр гаргаж болдоггүй хүснэгт бол дэлгэцийн зураг л гэсэн үг.

**csv**: UTF-8 BOM-той. BOM байхгүй бол Windows дээрх Excel монгол толгойг
mojibake болгож уншина — тэр нь тайлангийн бүх агуулга.

Файлын нэр нь тайлангийн түлхүүр + огноо. Локалчилсан нэр биш: кирилл үсэгтэй
файлын нэр браузер, и-мэйл клиентээр жигд бус дамждаг.
