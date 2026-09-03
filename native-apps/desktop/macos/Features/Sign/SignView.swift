import SwiftUI
import AppKit
import CryptoKit
import UniformTypeIdentifiers

/// PDF гарын үсэг — web demo хуудастай (`web/src/app/demo/page.tsx`) яг ижил урсгал:
///   1. PDF сонгоод SHA-256 digest-ийг локал тооцно (stamp хийх ИЖИЛ байт)
///   2. POST /api/sign-pdf-start {etsi, digestB64, fileName} →
///      {sessionId, vc, pollToken} — утас руу PIN2 push илгээгдэнэ
///   3. GET /api/status?sessionId=&pollToken= poll → COMPLETE/OK
///   4. POST /api/sign-pdf-download (file + sessionId + pollToken) →
///      тамгалагдсан (PAdES/PKCS#7 + баталгаажуулах хуудас) PDF-ийг
///      Downloads-д хадгална
struct SignView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    @State private var phase: Phase = .idle
    @State private var pollTask: Task<Void, Never>?

    enum Phase: Equatable {
        case idle
        case uploading(filename: String)
        case waiting(sessionID: String, filename: String, hash: String, code: String)
        case completed(filename: String, savedURL: URL)
        case error(String)
    }


    var body: some View {
        EidScroll {
            EidPageHeader(
                title: loc.t("Nav_Sign"),
                subtitle: loc.pick("PDF файл сонгож, утсан дээрх e-ID Mongolia апп-аас PIN2-аар гарын үсэг зурна.", "Pick a PDF and approve with PIN2 in the e-ID Mongolia app on your phone.", "Выберите PDF и подтвердите подписание с помощью PIN2 в приложении e-ID Mongolia на телефоне.", "选择 PDF，并在手机上的 e-ID Mongolia 应用中使用 PIN2 确认签名。")
            )

            switch phase {
            case .idle:                                       pickCard
            case .uploading(let filename):                    uploadingCard(filename)
            case let .waiting(_, filename, hash, code):       waitingCard(filename: filename, hash: hash, code: code)
            case let .completed(filename, savedURL):          completedCard(filename: filename, savedURL: savedURL)
            case .error(let msg):                             errorCard(msg)
            }
        }
        .onDisappear { pollTask?.cancel() }
    }

    // MARK: - Idle (file picker)

    private var pickCard: some View {
        AppCard {
            VStack(spacing: 14) {
                Image(systemName: "arrow.up.doc.fill")
                    .font(.system(size: 30, weight: .semibold))
                    .foregroundStyle(Color.eidAccent)
                Text(loc.pick("PDF файл сонгох", "Choose a PDF", "Выберите PDF", "选择 PDF 文件"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.pick("25 MB хүртэл, зөвхөн PDF", "Up to 25 MB, PDF only", "До 25 МБ, только PDF", "最大 25 MB，仅限 PDF"))
                    .labelStyle()
                Button {
                    pickAndSign()
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "signature")
                        Text(loc.pick("Файл сонгох", "Select file", "Выбрать файл", "选择文件"))
                    }
                }
                .buttonStyle(.primary)
                .disabled(signerEtsi.isEmpty)

                if signerEtsi.isEmpty {
                    Text(loc.pick("Гарын үсэг зурахын өмнө нэвтэрсэн байх шаардлагатай.", "You must be signed in before signing.", "Перед подписанием необходимо войти.", "签名前请先登录。"))
                        .font(.eidLabel)
                        .foregroundStyle(Color.eidDanger)
                }
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 18)
        }
    }

    // MARK: - Uploading

    private func uploadingCard(_ filename: String) -> some View {
        AppCard {
            HStack(spacing: 14) {
                ProgressView().controlSize(.regular)
                VStack(alignment: .leading, spacing: 2) {
                    Text(loc.pick("Илгээж байна…", "Uploading…", "Загрузка…", "上传中…"))
                        .font(.eidSectionTitle)
                        .foregroundStyle(Color.textPrimary)
                    Text(filename).labelStyle().lineLimit(1).truncationMode(.middle)
                }
                Spacer(minLength: 0)
            }
        }
    }

    // MARK: - Waiting (verification code + poll)

    private func waitingCard(filename: String, hash: String, code: String) -> some View {
        AppCard {
            VStack(spacing: 16) {
                Text(loc.pick("БАТАЛГААЖУУЛАХ КОД", "VERIFICATION CODE", "КОД ПОДТВЕРЖДЕНИЯ", "验证码"))
                    .font(.eidLabel)
                    .tracking(2)
                    .foregroundStyle(Color.eidMuted)

                VerificationCodeRow(code: (code.isEmpty ? "0000" : code))

                Text(loc.pick("Утсан дээрх e-ID Mongolia апп-д ижил кодтой эсэхийг шалгаад PIN2-оор баталгаажуулна уу.", "Check the same code shows in the e-ID Mongolia app, then approve with PIN2.", "Убедитесь, что в приложении e-ID Mongolia показан тот же код, затем подтвердите с помощью PIN2.", "请确认 e-ID Mongolia 应用中显示相同的验证码，然后使用 PIN2 确认。"))
                    .font(.eidBody)
                    .foregroundStyle(Color.textSecondary)
                    .multilineTextAlignment(.center)
                    .frame(maxWidth: 360)

                Divider()

                VStack(alignment: .leading, spacing: 6) {
                    metaRow(loc.pick("Файл", "File", "Файл", "文件"), filename, mono: false)
                    metaRow("SHA-256", hash, mono: true)
                }
                .frame(maxWidth: .infinity, alignment: .leading)

                HStack(spacing: 6) {
                    ProgressView().controlSize(.small)
                    Text(loc.pick("Хүлээж байна…", "Waiting…", "Ожидание…", "等待中…")).labelStyle()
                }

                Button(loc.pick("Цуцлах", "Cancel", "Отмена", "取消")) { cancel() }
                    .buttonStyle(.secondary)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 8)
        }
    }

    private func metaRow(_ label: String, _ value: String, mono: Bool) -> some View {
        HStack(alignment: .top, spacing: 10) {
            Text(label)
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
                .frame(width: 70, alignment: .leading)
            Text(value)
                .font(mono ? .eidMonoSmall : .eidBody)
                .foregroundStyle(Color.textSecondary)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
    }

    // MARK: - Completed

    private func completedCard(filename: String, savedURL: URL) -> some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                HStack(spacing: 8) {
                    Image(systemName: "checkmark.seal.fill")
                        .foregroundStyle(Color.eidSuccess)
                    Text(loc.pick("Гарын үсэг амжилттай", "Signed successfully", "Успешно подписано", "签署成功"))
                        .font(.eidSectionTitle)
                        .foregroundStyle(Color.eidSuccess)
                }
                Text(savedURL.lastPathComponent)
                    .font(.eidBody)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.pick("Downloads хавтаст хадгаллаа.", "Saved to your Downloads folder.", "Сохранено в папку «Загрузки».", "已保存到“下载”文件夹。"))
                    .labelStyle()

                HStack(spacing: 10) {
                    Button {
                        NSWorkspace.shared.activateFileViewerSelecting([savedURL])
                    } label: {
                        HStack(spacing: 6) {
                            Image(systemName: "folder")
                            Text(loc.pick("Finder-т харах", "Reveal in Finder", "Показать в Finder", "在访达中显示"))
                        }
                    }
                    .buttonStyle(.primary)

                    Button(loc.pick("Шинэ файл", "New file", "Новый файл", "新建文件")) { reset() }
                        .buttonStyle(.secondary)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // MARK: - Error

    private func errorCard(_ msg: String) -> some View {
        VStack(spacing: 12) {
            InlineBanner(text: msg, variant: .error)
            HStack {
                Button(loc.pick("Дахин оролдох", "Try again", "Повторить", "重试")) { reset() }
                    .buttonStyle(.secondary)
                Spacer()
            }
        }
    }

    // MARK: - Flow

    /// Гарын үсэг зурагчийн identifier — web demo-гийн `etsi` талбар
    /// (иргэний дугаар эсвэл РД, аль нэвтрэлтээс олдсоныг ашиглана).
    private var signerEtsi: String {
        let civil = appState.civilID.trimmingCharacters(in: .whitespaces)
        if !civil.isEmpty { return civil }
        return appState.nationalID.trimmingCharacters(in: .whitespaces)
    }

    /// Pick a PDF, then immediately start the signing session.
    private func pickAndSign() {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [.pdf]
        panel.message = loc.pick("Гарын үсэг зурах PDF файл сонгоно уу", "Choose a PDF to sign", "Выберите PDF для подписания", "请选择要签署的 PDF")
        guard panel.runModal() == .OK, let url = panel.url else { return }

        // SEC-3: refuse to drive the signing path on a tampered runtime.
        SecurityGuard.enforce()

        let filename = url.lastPathComponent
        phase = .uploading(filename: filename)
        pollTask?.cancel()
        pollTask = Task { await runSignFlow(fileURL: url, filename: filename) }
    }

    private func runSignFlow(fileURL: URL, filename: String) async {
        do {
            let pdfData = try Data(contentsOf: fileURL)
            guard pdfData.count <= 25 * 1024 * 1024 else {
                phase = .error(loc.pick("Файл 25 MB-аас том байна.", "File is larger than 25 MB.", "Файл больше 25 МБ.", "文件超过 25 MB。"))
                return
            }

            // Web demo-тэй ижил: эх PDF-ийн SHA-256-г клиент дээр тооцно
            // (stamp хийх ИЖИЛ байт) — энэ digest-ийг утас PIN2-оор зурна.
            let digest = SHA256.hash(data: pdfData)
            let digestB64 = Data(digest).base64EncodedString()
            let digestHex = digest.map { String(format: "%02x", $0) }.joined()

            let session: SignPdfStartResponse = try await APIClient.shared.request(
                .signPdfStart(etsi: signerEtsi.uppercased(), digestB64: digestB64, fileName: filename))
            let pollToken = session.pollToken ?? ""

            phase = .waiting(
                sessionID: session.sessionId,
                filename: filename,
                hash: digestHex,
                code: session.vc ?? ""
            )

            // Auth-тай ижил /api/status poll — COMPLETE хүртэл.
            let status = try await APIClient.shared.waitForAuth(
                sessionID: session.sessionId, pollToken: pollToken)
            guard status.isOK else {
                appState.recordSession(type: "SIGN", state: status.state ?? "COMPLETE",
                                       result: status.endResult ?? "FAILED",
                                       pushText: filename)
                phase = .error(Self.failureMessage(status, loc: loc))
                return
            }

            // Тамгалагдсан (PAdES + баталгаажуулах хуудас) PDF-ийг татна.
            let stamped = try await APIClient.shared.downloadStampedPdf(
                sessionID: session.sessionId, pollToken: pollToken,
                pdfData: pdfData, fileName: filename)
            let savedURL = try save(data: stamped, originalName: filename)

            appState.recordSession(type: "SIGN", state: "COMPLETE", result: "OK",
                                   pushText: filename)
            phase = .completed(filename: filename, savedURL: savedURL)
        } catch is CancellationError {
            // user cancelled
        } catch {
            phase = .error(error.localizedDescription)
        }
    }

    private static func failureMessage(_ status: StatusResponse, loc: LocalizationService) -> String {
        if let err = status.error, !err.isEmpty { return err }
        switch status.endResult ?? "" {
        case "USER_REFUSED", "USER_REFUSED_INTERACTION":
            return loc.pick("Гарын үсэг зурахаас татгалзлаа.", "Signing was declined.", "Подписание отклонено.", "签名已被拒绝。")
        case "TIMEOUT":
            return loc.pick("Хугацаа дууссан. Дахин оролдоно уу.", "Session expired. Please try again.", "Сессия истекла. Повторите попытку.", "会话已过期，请重试。")
        default:
            let detail = status.endResult ?? status.state ?? ""
            return loc.pick("Гарын үсэг амжилтгүй (\(detail)).",
                            "Signing failed (\(detail)).",
                            "Не удалось подписать (\(detail)).",
                            "签名失败（\(detail)）。")
        }
    }

    /// Persist the stamped bytes into ~/Downloads as `<original>_signed.pdf`.
    /// De-duplicates with a numeric suffix.
    private func save(data: Data, originalName: String) throws -> URL {
        let downloads = try FileManager.default.url(
            for: .downloadsDirectory, in: .userDomainMask, appropriateFor: nil, create: true)

        let base = (originalName as NSString).deletingPathExtension
        let chosen = "\(base)_signed.pdf"

        var target = downloads.appendingPathComponent(chosen)
        var n = 1
        while FileManager.default.fileExists(atPath: target.path) {
            target = downloads.appendingPathComponent("\(base)_signed (\(n)).pdf")
            n += 1
        }
        try data.write(to: target, options: .atomic)
        return target
    }

    private func cancel() {
        pollTask?.cancel()
        reset()
    }

    private func reset() {
        pollTask?.cancel()
        pollTask = nil
        phase = .idle
    }
}
