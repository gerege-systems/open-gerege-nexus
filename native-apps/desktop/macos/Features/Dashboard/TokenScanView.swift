import SwiftUI
import GeregeTokenKit
import SecurityInterface

/// "Токен уншигч" — Windows клиентийн `TokensPage`-ийн macOS хувилбар.
///
/// Windows дээр токен чөлөөтэй уншигддагийн шалтгаан нь ХОЁР backend зэрэг ажилладагт
/// байгаа: PKCS#11 (олдсон модуль тутамд нэг provider) ба CNG (minidriver-ийн системийн
/// санд нийтэлсэн объект). macOS-д CNG байхгүй тул түүний оронд ГЭРЭГЭ-гийн APDU зам
/// (BioPassDriver + Secure Messaging) хоёр дахь backend болно — OpenSC энэ картыг
/// PKCS#15 гэж бинд хийж чаддаггүй (`pkcs15-tool --dump` → "Unsupported card").
///
/// Дэлгэц нь Windows-тэйгээ ижилхэн: илэрсэн backend бүрийг мөр болгон харуулж, доор нь
/// уншигдсан гэрчилгээнүүдийг жагсаана.
struct TokenScanView: View {
    @ObservedObject private var loc = LocalizationService.shared

    @State private var backends: [BackendRow] = []
    @State private var certs: [CertRow] = []
    @State private var pin = ""
    @State private var scanning = false
    @State private var reading = false
    @State private var status = ""
    @State private var statusIsError = false

    /// Илэрсэн backend — PKCS#11 модуль эсвэл Гэрэгэ APDU.
    struct BackendRow: Identifiable {
        let id: String
        let name: String
        let detail: String
        let slots: Int
        let isApdu: Bool
    }

    /// Уншигдсан нэг гэрчилгээ.
    struct CertRow: Identifiable {
        let id: String
        let fileID: String
        let subject: String
        let issuer: String
        let serial: String
        let notAfter: String
        let bytes: Int
        let der: Data
    }

    var body: some View {
        EidScroll {
            EidPageHeader(
                title: loc.pick("Токен уншигч", "Token reader", "Считыватель токена", "令牌读取器"),
                subtitle: loc.pick("Илэрсэн backend болон токен дээрх гэрчилгээнүүд",
                                   "Detected backends and the certificates on the token",
                                   "Обнаруженные бэкенды и сертификаты на токене",
                                   "已检测到的后端与令牌上的证书"))
            backendsCard
            certificatesCard
            if !status.isEmpty {
                InlineBanner(text: status, variant: statusIsError ? .error : .info)
            }
        }
        .task { await scan() }
    }

    // MARK: - Backends (Windows: "Provider" section)

    private var backendsCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Text(loc.pick("Backend-үүд", "Backends", "Бэкенды", "后端"))
                        .font(.system(size: 14, weight: .semibold)).foregroundStyle(Color.textPrimary)
                    Spacer()
                    Button {
                        Task { await scan() }
                    } label: {
                        HStack(spacing: 6) {
                            if scanning { ProgressView().controlSize(.small) }
                            Image(systemName: "arrow.clockwise").font(.system(size: 11))
                            Text(loc.pick("Дахин хайх", "Rescan", "Пересканировать", "重新扫描"))
                        }
                    }
                    .disabled(scanning)
                }
                if backends.isEmpty {
                    Text(loc.pick("Backend олдсонгүй — уншигч залгаагүй эсвэл PKCS#11 модуль суугаагүй.",
                                  "No backend found — no reader attached, or no PKCS#11 module installed.",
                                  "Бэкенд не найден — считыватель не подключён или модуль PKCS#11 не установлен.",
                                  "未找到后端——未连接读卡器，或未安装 PKCS#11 模块。"))
                        .font(.system(size: 13)).foregroundStyle(Color.textSecondary)
                } else {
                    ForEach(backends) { b in backendRow(b) }
                }
            }
        }
    }

    private func backendRow(_ b: BackendRow) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: b.isApdu ? "cable.connector" : "shippingbox")
                .font(.system(size: 15))
                .foregroundStyle(Color.eidAccent)
                .frame(width: 34, height: 34)
                .background(Color.eidAccent.opacity(0.12))
                .clipShape(RoundedRectangle(cornerRadius: 9, style: .continuous))
            VStack(alignment: .leading, spacing: 3) {
                HStack(spacing: 8) {
                    Text(b.name).font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Color.textPrimary)
                    StatusPill(b.isApdu ? "APDU" : "PKCS#11", variant: b.isApdu ? .accent : .ok)
                }
                Text(b.detail).font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(Color.textSecondary).textSelection(.enabled)
            }
            Spacer(minLength: 0)
            Text(loc.pick("\(b.slots) токен", "\(b.slots) token(s)", "\(b.slots) токен(ов)", "\(b.slots) 个令牌"))
                .font(.system(size: 12)).foregroundStyle(Color.textSecondary)
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(Color.eidCardStroke, lineWidth: 1))
    }

    // MARK: - Certificates

    private var certificatesCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.pick("Токен дээрх гэрчилгээ", "Certificates on the token",
                              "Сертификаты на токене", "令牌上的证书"))
                    .font(.system(size: 14, weight: .semibold)).foregroundStyle(Color.textPrimary)
                HStack(spacing: 10) {
                    // PIN нь СОНГОЛТ: ESPK байршуулалттай карт (Tridium/Gerege Signature)
                    // гэрчилгээг PIN-гүйгээр өгдөг. Зөвхөн SM шаарддаг картад хэрэгтэй.
                    SecureField(loc.pick("Токен PIN (сонголт)", "Token PIN (optional)",
                                         "PIN токена (необязательно)", "令牌 PIN（可选）"), text: $pin)
                        .textFieldStyle(.roundedBorder)
                        .frame(maxWidth: 220)
                    Button {
                        Task { await readCertificates() }
                    } label: {
                        HStack(spacing: 6) {
                            if reading { ProgressView().controlSize(.small) }
                            Image(systemName: "doc.text.magnifyingglass").font(.system(size: 11))
                            Text(loc.pick("Гэрчилгээ унших", "Read certificates",
                                          "Прочитать сертификаты", "读取证书"))
                        }
                    }
                    .disabled(reading)
                }
                if certs.isEmpty {
                    Text(loc.pick("Гэрчилгээ уншаагүй байна.", "No certificates read yet.",
                                  "Сертификаты ещё не прочитаны.", "尚未读取证书。"))
                        .font(.system(size: 13)).foregroundStyle(Color.textSecondary)
                } else {
                    ForEach(certs) { c in certRow(c) }
                }
            }
        }
    }

    private func certRow(_ c: CertRow) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Image(systemName: "seal").font(.system(size: 14)).foregroundStyle(Color.eidSuccess)
                Text(c.subject).font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(Color.textPrimary).lineLimit(1)
                Spacer()
                StatusPill("EF \(c.fileID)", variant: .accent)
            }
            Text(loc.pick("Олгогч: ", "Issuer: ", "Издатель: ", "签发者：") + c.issuer)
                .font(.system(size: 11)).foregroundStyle(Color.textSecondary).lineLimit(1)
            HStack(spacing: 16) {
                Text("SN " + c.serial).font(.system(size: 11, design: .monospaced))
                Text(loc.pick("Дуусах: ", "Expires: ", "Истекает: ", "到期：") + c.notAfter)
                    .font(.system(size: 11))
                Text("\(c.bytes) B").font(.system(size: 11, design: .monospaced))
                Spacer()
                Button(loc.pick("Дэлгэрэнгүй", "Details", "Подробнее", "详情")) { showCertificate(c) }
                    .buttonStyle(.link)
                Button(loc.pick("PEM хадгалах", "Save PEM", "Сохранить PEM", "保存 PEM")) { savePem(c) }
                    .buttonStyle(.link)
            }
            .foregroundStyle(Color.textSecondary)
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(Color.eidCardStroke, lineWidth: 1))
    }

    // MARK: - Actions

    /// Windows-ийн ScanTokens: PKCS#11 модуль бүр + APDU замыг тусад нь мөр болгоно.
    private func scan() async {
        scanning = true; status = ""; statusIsError = false
        defer { scanning = false }

        var rows: [BackendRow] = []
        for (path, module) in GeregeTokenKit.PKCS11Module.openAll() {
            var slots = 0
            if (try? module.initialize()) != nil {
                slots = (try? module.getSlotList(tokenPresent: true))?.count ?? 0
                module.finalize()
            }
            rows.append(BackendRow(id: path, name: (path as NSString).lastPathComponent,
                                   detail: path, slots: slots, isApdu: false))
        }
        // Гэрэгэ-гийн APDU зам — CryptoTokenKit уншигч.
        if let info = try? await TokenProvisioner().getTokenInfo() {
            rows.append(BackendRow(id: "apdu", name: info.label,
                                   detail: "ATR \(info.atrHex)", slots: 1, isApdu: true))
        }
        backends = rows
        if rows.isEmpty {
            status = loc.pick("Уншигч ч, PKCS#11 модуль ч олдсонгүй.",
                              "Neither a reader nor a PKCS#11 module was found.",
                              "Не найдены ни считыватель, ни модуль PKCS#11.",
                              "既未找到读卡器，也未找到 PKCS#11 模块。")
            statusIsError = true
        }
    }

    private func readCertificates() async {
        reading = true; status = ""; statusIsError = false
        defer { reading = false }

        let (found, trace) = await TokenProvisioner().readCertificatesTraced(pin: pin)
        let svc = CertificateServiceImpl()
        certs = found.map { item in
            let data = Data(item.der)
            let fid = String(format: "%04X", item.fileID)
            let b64 = data.base64EncodedString(options: [.lineLength64Characters, .endLineWithLineFeed])
            let pem = "-----BEGIN CERTIFICATE-----\n\(b64)\n-----END CERTIFICATE-----\n"
            if case .success(let info) = svc.parsePem(pem) {
                let f = DateFormatter(); f.dateFormat = "yyyy.MM.dd"
                return CertRow(id: fid, fileID: fid, subject: info.subject, issuer: info.issuer,
                               serial: info.serialNumberHex, notAfter: f.string(from: info.notAfter),
                               bytes: data.count, der: data)
            }
            return CertRow(id: fid, fileID: fid, subject: "(задлагдсангүй)", issuer: "—",
                           serial: "—", notAfter: "—", bytes: data.count, der: data)
        }

        if certs.isEmpty {
            statusIsError = true
            status = loc.pick("Гэрчилгээ олдсонгүй — ", "No certificate found — ",
                              "Сертификаты не найдены — ", "未找到证书 — ")
                + trace.joined(separator: " · ")
                + " · " + (await diagnosticText())
        } else {
            status = loc.pick("\(certs.count) гэрчилгээ уншлаа.", "Read \(certs.count) certificate(s).",
                              "Прочитано сертификатов: \(certs.count).", "已读取 \(certs.count) 张证书。")
                + " (" + trace.joined(separator: " · ") + ")"
        }
    }

    /// Гэрчилгээ олдоогүй/алдаа гарсан үед картын CDF-ийн түүхий байтыг харуулна —
    /// гэрчилгээний бодит EF ID-г эндээс тодруулж сканнердах мужийг тааруулна.
    private func diagnosticText() async -> String {
        let files = await TokenProvisioner().readDiagnosticFiles()
        guard !files.isEmpty else { return "(CDF ч уншигдсангүй)" }
        return files.map { f in
            "EF " + String(format: "%04X", f.fileID) + ": "
                + f.bytes.map { String(format: "%02X", $0) }.joined()
        }.joined(separator: " | ")
    }

    /// Гэрчилгээг ГИНЖТЭЙ нь macOS-ийн өөрийн цонхоор харуулна (SFCertificatePanel) —
    /// Keychain Access-ийнхтэй яг ижил: Subject/Issuer-ийн бүх RDN, серийн дугаар,
    /// хувилбар, гарын үсгийн алгоритм, хугацаа, нийтийн түлхүүр, Key Usage, итгэлцэл.
    ///
    /// Гинжийг ӨӨРСДӨӨ угсрахгүй: SecTrust нь дутуу CA-г гэрчилгээн дэх AIA
    /// (Authority Information Access → CA Issuers) холбоосоор татаж, Keychain болон
    /// системийн anchor-уудаас нөхөж өгдөг. Итгэлцлийн ҮР ДҮН биш, ГИНЖ нь л
    /// хэрэгтэй тул evaluate-ийн буцаалтыг үл хэрэгснэ (Tridium/Gerege-гийн CA
    /// системийн санд байхгүй нь хэвийн).
    private func showCertificate(_ c: CertRow) {
        guard let leaf = SecCertificateCreateWithData(nil, c.der as CFData) else {
            status = loc.pick("Гэрчилгээг задалж чадсангүй.", "Could not parse the certificate.",
                              "Не удалось разобрать сертификат.", "无法解析该证书。")
            statusIsError = true
            return
        }
        // AIA нь СҮЛЖЭЭНИЙ хүсэлт — main thread дээр гүйцэтгэвэл UI хэдэн секунд гацна.
        status = loc.pick("Гинжийг угсарч байна…", "Building the issuer chain…",
                          "Построение цепочки…", "正在构建证书链…")
        statusIsError = false
        Task {
            // Тэргүүлэх зэрэг нь ЗААВАЛ: тэмдэглээгүй `Task.detached` нь дунд
            // зэргээр гүйдэг тул түүнийг хүлээж буй UI (user-interactive) утас
            // өөрөөсөө ДООГУУР зэрэгтэй ажил хүлээх болж, Thread Performance
            // Checker «Hang Risk — waiting on a lower QoS» гэж бүртгэдэг.
            // Энэ модуль дахь бусад detached дуудлагууд аль хэдийн ингэсэн.
            let chain = await Task.detached(priority: .userInitiated) { certificateChain(for: leaf) }.value
            if chain.count == 1 {
                // AIA байхгүй эсвэл CA татагдаагүй — ганц гэрчилгээг харуулна.
                status = loc.pick(
                    "Гинжний дээд түвшин олдсонгүй (гэрчилгээнд AIA заагаагүй эсвэл CA татагдсангүй) — зөвхөн энэ гэрчилгээг харуулж байна.",
                    "Issuer chain unavailable (no AIA in the certificate, or the CA could not be fetched) — showing this certificate only.",
                    "Цепочка издателей недоступна (нет AIA или CA не загружен) — показан только этот сертификат.",
                    "无法获取签发链（证书中无 AIA，或 CA 下载失败）——仅显示该证书。")
                statusIsError = false
            } else {
                status = loc.pick("Гинж: \(chain.count) гэрчилгээ.", "Chain: \(chain.count) certificates.",
                                  "Цепочка: \(chain.count) сертификата.", "证书链：\(chain.count) 张。")
                statusIsError = false
            }
            SFCertificatePanel.shared()?.beginSheet(
                for: NSApp.keyWindow, modalDelegate: nil, didEnd: nil, contextInfo: nil,
                certificates: chain, showGroup: true)
        }
    }

    private func savePem(_ c: CertRow) {
        let panel = NSSavePanel()
        panel.nameFieldStringValue = "token-\(c.fileID).pem"
        guard panel.runModal() == .OK, let url = panel.url else { return }
        let b64 = c.der.base64EncodedString(options: [.lineLength64Characters, .endLineWithLineFeed])
        let pem = "-----BEGIN CERTIFICATE-----\n\(b64)\n-----END CERTIFICATE-----\n"
        try? pem.write(to: url, atomically: true, encoding: .utf8)
    }
}

/// Leaf-ээс эхлэн боломжит гинжийг угсарна (AIA татах зөвшөөрөлтэй).
///
/// `SecTrustEvaluateWithError` нь AIA-гийн CA-г ТАТАХ үедээ блоклодог тул энэ функцыг
/// зөвхөн main thread-ээс ГАДУУР (`Task.detached`) дуудна. Struct-ын гадна байгаа нь
/// санамсаргүй `self`-ийг detached контекст рүү зөөхөөс сэргийлж байгаа хэрэг.
private func certificateChain(for leaf: SecCertificate) -> [SecCertificate] {
    var trust: SecTrust?
    let policy = SecPolicyCreateBasicX509()
    guard SecTrustCreateWithCertificates(leaf as CFTypeRef, policy, &trust) == errSecSuccess,
          let trust else { return [leaf] }
    SecTrustSetNetworkFetchAllowed(trust, true)
    _ = SecTrustEvaluateWithError(trust, nil)   // үр дүн хамаагүй — гинж л хэрэгтэй
    guard let chain = SecTrustCopyCertificateChain(trust) as? [SecCertificate],
          !chain.isEmpty else { return [leaf] }
    return chain
}
