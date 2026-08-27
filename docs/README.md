# Баримт бичгийн төв — Documentation Hub

> Одоогийн нийтлэх баримтууд вэб хэлбэрээр
> **[gerege-systems.github.io/open-gerege-nexus](https://gerege-systems.github.io/open-gerege-nexus/)**
> хаяг дээр нийтлэгддэг. Сайт нь энэ хавтасны Markdown файлуудаас
> угсрагддаг тул эх сурвалж нь энд хэвээр байна —
> [`docs/site/`](site/) хавтсыг үзнэ үү.

Энэ хавтас нь **Gerege Nexus**-ын баримт бичиг, түүхэн design record болон
байгаа орчуулгуудыг агуулна. Үндсэн хэл нь монгол; орчуулга байгаа файл
`_AR`, `_ZH`, `_EN`, `_FR`, `_RU`, `_ES` дагавар хэрэглэнэ. Бүтээгдэхүүний
тойм долоон хэлээр бүрэн; техникийн баримт бүр заавал долоон хувилбартай биш.

**Хэлний бодлого: монгол хэл + НҮБ-ын албан ёсны 6 хэл** (араб, хятад, англи,
франц, орос, испани) — нийт 7 хэл. Монгол хэл эх сурвалж, бусад нь орчуулга.
Шинэ хэл нэмэхийн өмнө энэ бодлогыг өөрчлөх шаардлагатай: жагсаалт нь дур
зоргоор биш, олон улсын байгууллагуудын хэрэглэдэг жишигт тулгуурласан.

This directory holds Gerege Nexus documentation, historical design records,
and the translations that exist. Mongolian is the source language. The product
overview follows the language policy of Mongolian plus the six official UN
languages; individual technical documents may remain in their source language.

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
  <a href="README_ES.md"><img src="assets/icons/flag-es.png" width="18" height="18" alt=""> Español</a>
</p>

---

## Танилцуулга — Overview

| Хэл | Файл |
| --- | --- |
| Монгол (эх сурвалж) | [`../README.md`](../README.md) |
| العربية | [`README_AR.md`](README_AR.md) |
| 中文 | [`README_ZH.md`](README_ZH.md) |
| English | [`README_EN.md`](README_EN.md) |
| Français | [`README_FR.md`](README_FR.md) |
| Русский | [`README_RU.md`](README_RU.md) |
| Español | [`README_ES.md`](README_ES.md) |

## Одоогийн ажиллагааны баримт — Current documentation

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`ARCHITECTURE_SPECIFICATION.md`](ARCHITECTURE_SPECIFICATION.md) | MN | Платформын давхаргууд, өгөгдлийн загвар, архитектурын шийдвэрүүд |
| [`ARCHITECTURE_SPECIFICATION_EN.md`](ARCHITECTURE_SPECIFICATION_EN.md) | EN | Architecture specification |
| [`SSO_FEDERATION.md`](SSO_FEDERATION.md) | MN | Нэг суулгацыг нөгөөгийн SSO клиент болгох: env, урсгал, гарах зам |
| [`SHELL_CONTRACT.md`](SHELL_CONTRACT.md) | MN | Native бүрхүүл ба web ажлын мужийн `window.GeregeShell` гэрээ |
| [`NATIVE_LOGIN_SPEC.md`](NATIVE_LOGIN_SPEC.md) | MN | Swift, C#, Kotlin клиентүүдийн нэвтрэлтийн зан төлөв |
| [`NATIVE_SETTINGS_SPEC.md`](NATIVE_SETTINGS_SPEC.md) | MN | Бүрхүүл, төхөөрөмж, peripheral, fleet тохиргоо |
| [`MODULE_AUTHORING_GUIDE.md`](MODULE_AUTHORING_GUIDE.md) | EN | Шинэ апп модуль хөгжүүлэх алхам алхмаар заавар |
| [`RELEASING.md`](RELEASING.md) | MN | `pkg/nexus`-ийн semver амлалт, түүнийг хамгаалдаг тест, tag гаргах журам |
| [`GOV_SERVICES_WORKFLOW.md`](GOV_SERVICES_WORKFLOW.md) | EN | `gov-gerege-nexus` distribution-д байдаг төрийн үйлчилгээний модулийн reference |
| [`DOCUMENTS_SIGNING.md`](DOCUMENTS_SIGNING.md) | EN | `client-gerege-nexus` distribution-ийн Documents модуль ба core signing rail-ийн хил |
| [`APPSTORE_OPERATIONS.md`](APPSTORE_OPERATIONS.md) | EN | Апп сторын каталог нийтлэх, хувилбар шилжүүлэх ажиллагаа |
| [`REPORTS.md`](REPORTS.md) | MN | Core тайлангийн хөдөлгүүр ба `client-gerege-nexus`-ийн report UI модулийн хил |
| [`REPORT_SHARING.md`](REPORT_SHARING.md) | MN | Тенант дамнасан тайлан: grant, counterparty хүрээ, хоёр талын audit |
| [`URTUU.md`](https://github.com/gerege-systems/client-gerege-nexus/blob/main/docs/URTUU.md) | MN | **Энэ репод байхаа больсон.** Өртөө — суваг ба самбар хоёул `client-gerege-nexus`-д |
| [`RING_STANDARD.md`](https://github.com/gerege-systems/client-gerege-nexus/blob/main/docs/RING_STANDARD.md) | MN | **Энэ репод байхаа больсон.** Кодын бүртгэлийн формат Өртөөгийн хамт явав |
| [`MONITORING.md`](MONITORING.md) | MN | Ажиглалтын стек: асаах, Grafana, лог хайх, шинэ хэмжүүр нэмэх |
| [`RUNBOOKS.md`](RUNBOOKS.md) | MN | Дохио бүрд: юу болсон, юу шалгах, яаж засах, хэзээ өргөжүүлэх |
| [`CONTROL_PLANE.md`](CONTROL_PLANE.md) | MN | Операторын консол: босгох, эрх, анхны оператор үүсгэх, аюулгүй байдлын дүрмүүд |
| [`TWO_PLANES_REVIEW.md`](TWO_PLANES_REVIEW.md) | MN | Хоёр урсгалын хэрэгжсэн төлөв, баталгаа, үлдсэн backlog |
| [`TRANSLATION_GUIDE.md`](TRANSLATION_GUIDE.md) | MN | Долоон хэлний толь бичиг, орчуулга нэмэх урсгал |

## Архитектурын шийдвэр — Architecture decisions

ADR нь кодын одоогийн хэлбэрийг **яагаад** сонгосныг тайлбарлана. Одоогийн
зан төлөвийг мэдэхдээ дээрх ажиллагааны баримтыг, шалтгааныг мэдэхдээ ADR-ыг
уншина.

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`ECOSYSTEM_GIT_STRATEGY.md`](ECOSYSTEM_GIT_STRATEGY.md) | MN | Цөм, distribution, каталог ба нийтийн SDK-ийн хил |
| [`adr/0001-domain-first.md`](adr/0001-domain-first.md) | MN | Аппын дүрэм платформоо импортлохгүй |
| [`adr/0002-one-signing-rail.md`](adr/0002-one-signing-rail.md) | MN | Гарын үсгийн зам нэг (`eidmongolia`) |
| [`adr/0003-a-document-carries-what-is-signed.md`](adr/0003-a-document-carries-what-is-signed.md) | MN | Баримт файлаа авч явна: pades, detached, approval |
| [`adr/0004-a-pilot-that-did-not-ship.md`](adr/0004-a-pilot-that-did-not-ship.md) | MN | Өртөөний distribution pilot яагаад гараагүй вэ |
| [`adr/0005-two-planes-one-origin-each.md`](adr/0005-two-planes-one-origin-each.md) | MN | Нэг бинарь, хоёр origin: тенант ба операторын хаалга |
| [`adr/0006-a-person-owns-a-space.md`](adr/0006-a-person-owns-a-space.md) | MN | Хүн муж эзэмшинэ: `workspace`/`operator`/`registry` нэршил, гэр ба байгууллага, иргэний буулт (нэршил ба гэр хэрэгжсэн; хувийн муж **санал**) |
| [`adr/0007-a-rail-needs-a-second-caller.md`](adr/0007-a-rail-needs-a-second-caller.md) | MN | Рельс болохын тулд хоёр дахь дуудагч хэрэгтэй: Өртөө ба холбогч цөмөөс бүрэн гарсан шалтгаан |

## Санал, төлөвлөгөө, ажлын түүх — Historical design records

Эдгээр нь шийдвэр гаргах үеийн хэмжилт, хувилбар, хэрэгжүүлэлтийн дарааллыг
хадгална. **Одоогийн API, package зам, тохиргооны source of truth биш.** Файл
бүрийн status banner болон холбоосоор canonical баримт руу орно.

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`CORE_BOUNDARY_PLAN.md`](CORE_BOUNDARY_PLAN.md) | MN | Цөмийн хилийн хэмжилт ба хэрэгжсэн салгалтын төлөвлөгөө |
| [`TWO_PLANES_PROPOSAL.md`](TWO_PLANES_PROPOSAL.md) | MN | `tenant`/`platform`/`kernel`, хоёр schema-ийн анхны санал |
| [`WORKSPACE_NAMING_PROPOSAL.md`](WORKSPACE_NAMING_PROPOSAL.md) | MN | Нэршлийн засвар, хувийн орон зай, эрэлтийн тал (100 нийлүүлэгч), иргэний `/me` буулт — үе A–G (**A, B, C1-lite, E, F, G хэрэгжсэн**) |
| [`MONITORING_AND_REPORTING_PROPOSAL.md`](MONITORING_AND_REPORTING_PROPOSAL.md) | MN | Ажиглалт ба тайлангийн дизайны санал |
| [`URTUU_PROPOSAL.md`](https://github.com/gerege-systems/client-gerege-nexus/blob/main/docs/URTUU_PROPOSAL.md) | MN | **Энэ репод байхаа больсон.** «Өртөө» сувгийн анхны дизайны санал |
| [`CONTROL_PLANE_PLAN.md`](CONTROL_PLANE_PLAN.md) | MN | Операторын консолын анхны дизайн ба үе шатууд |
| [`PEER_PROPOSAL.md`](PEER_PROPOSAL.md) | MN | Nexus биш системийг Өртөөний талд оруулах санал |
| [`APPSTORE_SEPARATION_PLAN.md`](APPSTORE_SEPARATION_PLAN.md) | MN | App Store салгах төлөвлөгөө |
| [`APPSTORE_PHASE2_PLAN.md`](APPSTORE_PHASE2_PLAN.md) | MN | «Шастир» ба платформ нэгдлийн 2-р үе шат |
| [`DOCUMENTS_WORKLOG.md`](DOCUMENTS_WORKLOG.md) | MN | Баримт ба цахим гарын үсгийн ажлын түүх |

## Хэрэгжүүлэлтийн prompt-ууд — Implementation prompts

Хийгдсэн ажлыг хэрхэн даалгасныг үлдээсэн бичвэрүүд. Тэдгээр нь түүх:
prompt-ын төлөвлөгөө ба эцэст нь хэрэгжсэн зүйл заримдаа зөрдөг, тэр
зөрүү нь ADR-д тэмдэглэгддэг.

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`MODULE_RENAME_PROMPT.md`](MODULE_RENAME_PROMPT.md) | MN | Цөмийн модулиудын нэршлийн засвар (салгалтын 0-р алхам) |
| [`MONITORING_AND_REPORTING_IMPLEMENTATION_PROMPT.md`](MONITORING_AND_REPORTING_IMPLEMENTATION_PROMPT.md) | MN | Мониторинг ба тайлангийн системийг хэрэгжүүлэх prompt |
| [`DOMAIN_FIRST_PROMPT.md`](DOMAIN_FIRST_PROMPT.md) | MN | Домэйн давхаргыг салгах prompt — үр дүн нь [`adr/0001-domain-first.md`](adr/0001-domain-first.md) |
| [`CORE_BOUNDARY_PROMPTS.md`](CORE_BOUNDARY_PROMPTS.md) | MN | [`CORE_BOUNDARY_PLAN.md`](CORE_BOUNDARY_PLAN.md)-ыг хэрэгжүүлэх үе шат бүрийн prompt — арваулаа хэрэгжсэн, үр дүнгийн хүснэгттэй |
| [`TWO_PLANES_PROMPTS.md`](TWO_PLANES_PROMPTS.md) | MN | [`TWO_PLANES_PROPOSAL.md`](TWO_PLANES_PROPOSAL.md)-ыг хэрэгжүүлэх Үе A–H-ийн prompt |
| [`PERSON_PLANE_PROMPTS.md`](PERSON_PLANE_PROMPTS.md) | MN | Иргэний урсгал (`internal/person`) — P0–P3-ийн prompt: порт, `gerege_nexus_person` role, policy, `/me` (**хэрэгжээгүй**; §«Гурван засвар» — P2 нь модулийн репод) |

## Төслийн журам — Project governance

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`../CONTRIBUTING.md`](../CONTRIBUTING.md) | MN | Хувь нэмэр оруулах журам |
| [`CONTRIBUTING_EN.md`](CONTRIBUTING_EN.md) | EN | Contribution guide |
| [`../CODE_OF_CONDUCT.md`](../CODE_OF_CONDUCT.md) | MN | Ёс зүйн дүрэм |
| [`CODE_OF_CONDUCT_EN.md`](CODE_OF_CONDUCT_EN.md) | EN | Code of conduct |
| [`../SECURITY.md`](../SECURITY.md) | MN | Аюулгүй байдлын бодлого |
| [`SECURITY_EN.md`](SECURITY_EN.md) | EN | Security policy |
| [`../CHANGELOG.md`](../CHANGELOG.md) | EN | Өөрчлөлтийн түүх |

---

## Орчуулга нэмэх — Adding a translation

1. Эх баримтыг хуулж, файлын нэрэнд ISO 639-1 хэлний код бүхий дагавар нэмнэ:
   `README_JA.md`, `CONTRIBUTING_JA.md` гэх мэт.
2. Баримтын эхэнд, оршил догол мөрийн дараа, badge-үүдийн өмнө хэлний
   сонголтын мөрийг байрлуулна. Туг бүрийн зураг
   [`assets/icons/`](assets/icons/)-д хадгалагдана.
3. Бүх хэлний хувилбар дээрх сонголтын мөрийг шинэ хэлээр нөхнө — сонголт нь
   хэлүүдийн хооронд тэгш хэмтэй байх ёстой.
4. Энэ индекс файлын хүснэгтэд шинэ мөр нэмнэ.

Хэлний сонголтын мөрийн загвар (`docs/` доторх файлд):

```html
<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
</p>
```

Идэвхтэй байгаа хэлийг холбоосгүй, `<b>` тэгээр тодруулна.

---

## Дүрсний эх сурвалж — Icon source

Тугны дүрсийг [Flaticon](https://www.flaticon.com/)-оос авч репод хадгалсан.
Дэлгэрэнгүйг [`assets/icons/ATTRIBUTION.md`](assets/icons/ATTRIBUTION.md)-ээс
үзнэ үү. Баримт бичигт emoji дүрс ашиглахгүй — бүх дүрс нь Flaticon-ы дүрсийн
сангаас авсан зураг байна.
