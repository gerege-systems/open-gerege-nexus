# Хувь нэмэр оруулах заавар

**Gerege Nexus** (`open-gerege-nexus`) төсөлд хувь нэмэр оруулах
сонирхол гаргасанд баярлалаа. Модуль бүтэцтэй, өндөр бүтээмжтэй нээлттэй эхийн
платформыг хамтдаа бүтээхэд таны оролцоог урьж байна.

<p>
  <img src="docs/assets/icons/flag-mn.png" width="18" height="18" alt=""> <b>Монгол</b>
  &nbsp;·&nbsp;
  <a href="docs/CONTRIBUTING_EN.md"><img src="docs/assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
</p>

---

## Хариуцагчид

Төслийг дараах баг хөгжүүлж, хариуцан ажиллуулна:

- **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems))
- **Gemini AI**, **Claude AI**

---

## Ёс зүйн дүрэм

Хувь нэмэр оруулагч бүр [Ёс зүйн дүрэм](CODE_OF_CONDUCT.md)-ийг мөрдөнө.
Зохисгүй үйлдлийг `community@gerege.mn` хаягаар мэдэгдэнэ үү.

---

## Хэрхэн хувь нэмэр оруулах вэ

### 1. Алдаа мэдээлэх

Алдаа мэдээлэхийн өмнө нээлттэй issue-үүдийг шалгаж, давхардал үүсгэхээс
сэргийлнэ үү. Мэдээлэлдээ дараахыг заавал оруулна:

- Алдааг давтан гаргах тодорхой алхмууд.
- Ажиллаж буй орчин (Go хувилбар, Node.js хувилбар, үйлдлийн систем,
  PostgreSQL хувилбар).
- Хүлээгдэж буй ба бодит үр дүн, боломжтой бол лог.

### 2. Шинэ боломж санал болгох

Санал болгож буй боломжийн хэрэглээний тохиолдол, шийдэх гэж буй асуудал,
төсөөлж буй шийдлээ тодорхой бичнэ үү.

### 3. Pull request илгээх

1. **Салбар үүсгэх** — `git checkout -b feature/amazing-feature`.
2. **Код бичих хэв маягийг мөрдөх**:
   - Backend: Go 1.26+, `gofmt` форматлалт, `slog` бүтэцтэй логлолт, алдааг
     тодорхой шалгах.
   - Frontend: Next.js 16 App Router, TypeScript strict горим, Tailwind CSS.
3. **Тест бичих** — backend-д нэмэгдсэн логик бүрт `*_test.go` тест дагалдана.
4. **Шалгалтуудыг ажиллуулах**:

   ```bash
   # Backend: формат, статик шинжилгээ, тест
   cd backend
   gofmt -l .
   go vet ./...
   go test -race ./...
   golangci-lint run

   # Frontend: төрлийн шалгалт ба build
   cd ../frontend
   npx tsc --noEmit
   npm run build
   ```

5. **Commit бичлэг** — [Conventional Commits](https://www.conventionalcommits.org/)
   хэлбэрийг баримтална:
   - `feat: add invoice management module`
   - `fix: resolve stock level calculation rounding`
   - `docs: update module authoring guide`
6. **PR нээх** — `main` салбар руу чиглүүлнэ. CI дээрх lint, тест, frontend
   build, аюулгүй байдлын шалгалт бүгд ногоон байх шаардлагатай.

---

## Шинэ бизнес модуль нэмэх

1. `backend/internal/apps/<module_name>/` дор шинэ пакет үүсгэнэ.
2. `backend/pkg/nexus`-ийн `nexus.Module` интерфейсийг бүрэн
   хэрэгжүүлнэ.
3. Модулийг `nexus.Register`-ээр бүртгэж, `catalog/manifests/<slug>.json` manifest
   файл нэмнэ. Manifest-ийн бүтэц `appcatalog.Manifest`-тэй яг таарах ёстой —
   алдаатай manifest бол сервер асахгүй.
4. `catalog/apps.json`-д апп-аа бүртгэнэ. `apps` хүснэгт нь ачаалал бүрт энэ
   файлаас синк болох тул SQL гараар бичих шаардлагагүй.
5. `frontend/app/<module_name>/page.tsx` дор дэлгэц нэмнэ.

Дэлгэрэнгүйг [Модуль бичих](docs/MODULES.md)-ээс үзнэ үү.

---

## `backend/pkg/nexus`-д хүрэх PR

Энэ пакет бол экосистемийн гэрээ: distribution repo бүр түүн рүү компилл
хийж, tag-аар түгжинэ. Тиймээс:

- **CODEOWNERS-ийн review заавал** (`.github/CODEOWNERS`).
- Экспортолсон гадаргуу өөрчлөгдвөл `go test ./pkg/nexus -update` ажиллуулж,
  `testdata/api.txt`-ийн diff-ийг PR-даа оруулна. Тэр diff нь гэрээ юу авч,
  юу алдсаныг review-д яг хэлнэ.
- Эвдэх өөрчлөлт бол deprecation → нэг major цикл. Дүрмийг
  [Модуль бичих](docs/MODULES.md)-ийн «Хувилбарын амлалт» хэсгээс үзнэ үү.

---

## Баримт бичиг ба орчуулга

- Үндсэн хэл нь монгол. Орчуулгууд `docs/` дор `_EN`, `_ZH`, `_RU` дагаварт
  байрлана.
- Баримт бичигт **emoji ашиглахгүй**. Дүрс шаардлагатай бол
  [`docs/assets/icons/`](docs/assets/icons/) дахь Flaticon дүрсийг ашиглаж,
  [`ATTRIBUTION.md`](docs/assets/icons/ATTRIBUTION.md)-д эх сурвалжийг нэмнэ.
- Орчуулга нэмэх журмыг [`docs/README.md`](docs/README.md)-ээс үзнэ үү.
