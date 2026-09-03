import Foundation
import Network
import GeregeTokenKit
import os

/// ESIGN "программ токен" гүүр — Windows `RawWebSocketServer` + `EsignProtocol`-ийн macOS порт.
///
/// ДАН/sso.gov.mn-ий нэвтрэх хуудас `ws://127.0.0.1:59001` руу холбогдож дараах текст мессеж
/// илгээнэ:
/// ```
/// { "type":"bb4702f31917793f", "data":{ "_ott":"…", "reg-num":"…" }, "cert":"<b64 серверийн DER>" }
/// ```
/// Хариу: `{ "status":"success", "signature":…, "cipher":…, "meta":… }` эсвэл
/// `{ "status":"error", "message":… }`.
///
/// Физик токеноос ялгаатай нь гарын үсгийг **утас** зурна (threshold ECDSA, PIN2) — иймд
/// хүсэлт бүр утас руу push явуулж, хэрэглэгч баталгаажуулах кодыг тулгах ёстой. Тэр
/// кодыг `pending`-ээр UI-д гаргана.
///
/// RFC 6455-ийн handshake/framing-ийг `NWProtocolWebSocket` (Network.framework) хийнэ —
/// гараар socket бичихгүй.
///
/// ponytail: зөвхөн `ws://127.0.0.1:59001`. Windows гүүр `wss://…:59005`-ыг ч ажиллуулдаг
/// (өөрөө localhost cert үүсгэдэг); macOS дээр NWProtocolTLS нь SecIdentity шаарддаг ба
/// Security.framework-д self-sign хийх API байхгүй тул DER-ийг гараар угсрах хэрэгтэй болно.
/// Chrome/Edge нь `ws://127.0.0.1`-ыг https хуудаснаас зөвшөөрдөг тул эхний хувилбарт
/// хангалттай; Safari/Firefox шаардвал үүнийг нэмнэ.
@MainActor
final class EsignBridge: ObservableObject {

    static let shared = EsignBridge()
    static let port: UInt16 = 59001
    /// ДАН-ий `ENCRYPTED_DATA_SIGN` хүсэлтийн тогтмол `type` (sso.gov.mn дээр ажигласан).
    static let expectedType = "bb4702f31917793f"

    /// Утас руу push явуулсан, хэрэглэгчийн баталгаажуулалт хүлээж буй хүсэлт.
    struct Pending: Equatable {
        let verificationCode: String
        let regNum: String
    }

    /// USB токен замд UI-аас PIN хүлээж буй хүсэлт (утасны `pending`-тэй тэгш хэмтэй).
    struct PINRequest: Equatable {
        let regNum: String
    }

    @Published private(set) var isRunning = false
    @Published private(set) var pending: Pending?
    @Published private(set) var pinRequest: PINRequest?
    @Published private(set) var log: [String] = []

    /// Гарын үсэг зурах identity — нэвтэрсэн үед AppState өгнө, гарахад nil болно.
    private var identity: (personID: String, certificateB64: String)?
    private var listener: NWListener?
    /// ESPK токен driver (SM + PIN + PSO). Токен зам сонгогдоход л ашиглана.
    private let espkDriver = BioPassDriver()
    private let oslog = Logger(subsystem: "mn.gerege.nexus.desktop", category: "EsignBridge")
    private var pinContinuation: CheckedContinuation<String, Error>?

    enum BridgeError: LocalizedError {
        case pinCancelled
        case noTokenCert
        var errorDescription: String? {
            switch self {
            case .pinCancelled: return "PIN оруулахыг цуцаллаа."
            case .noTokenCert:  return "Токен дээр ESIGN гэрчилгээ олдсонгүй."
            }
        }
    }

    private init() {}

    // MARK: - USB токен PIN (UI-аас)

    /// UI PIN sheet-ээс дуудна — хүлээж буй токен гарын үсгийг үргэлжлүүлнэ.
    func providePIN(_ pin: String) {
        pinRequest = nil
        pinContinuation?.resume(returning: pin)
        pinContinuation = nil
    }

    /// UI PIN sheet-ийг цуцлав.
    func cancelPIN() {
        pinRequest = nil
        pinContinuation?.resume(throwing: BridgeError.pinCancelled)
        pinContinuation = nil
    }

    private func requestPIN(regNum: String) async throws -> String {
        pinContinuation?.resume(throwing: BridgeError.pinCancelled)  // өмнөх хүлээлт цэвэрлэнэ
        return try await withCheckedThrowingContinuation { c in
            pinContinuation = c
            pinRequest = PINRequest(regNum: regNum)
        }
    }

    /// ДАН-ий P payload-ийг USB ESPK токеноор гарын үсэглэнэ. P нь ТОКЕНЫ leaf cert-ийг
    /// агуулах ёстой тул нэг session дотор: SM → cert унших → P угсрах → PIN → PSO streaming.
    /// (signature, P) буцаана — P-г дараа нь server cert-ээр seal хийнэ.
    private func signViaToken(dataJSON: String, regNum: String) async throws -> (signature: Data, payload: Data) {
        let pin = try await requestPIN(regNum: regNum)
        // ponytail: гарын үсгийн түлхүүр (MSE:SET `A0 20`) Windows трэйс дээр 24C0 (Tridium)
        // leaf-ийн pubkey-тэй тохирсон. Өөр байршуулалттай токенд leaf сонголтыг тохируул.
        let signingFileID: UInt16 = 0x24C0
        return try await GeregeTokenKit.TokenManager.shared.withSession { [espkDriver] card in
            try await espkDriver.establishSecureSession(card: card)
            let certs = try await espkDriver.readESPKCertificates(card: card)
            guard let leaf = certs.first(where: { $0.fileID == signingFileID })?.der ?? certs.first?.der else {
                throw BridgeError.noTokenCert
            }
            let leafData = Data(leaf)
            let p = EsignCrypto.payload(
                dataJSON: dataJSON,
                certificateB64: leafData.base64EncodedString(),
                sn: EsignCrypto.certificateSerial(leafData),
                keyIDB64: EsignCrypto.certificateKeyID(leafData).base64EncodedString())
            // readESPKCertificates сүүлийн EF-д зогсдог тул DF 2003-ыг дахин сонгоно.
            // Windows трэйс (node 68) нь гарын үсгийн өмнө ЯГ нэг `SELECT 00 A4 00 00 2003`
            // хийдэг (MF сонголтгүй) — тэр нь DF-түвшинд буцаадаг тул PIN/PSO-д хангалттай.
            // (Трэйсийн GET DATA 84/81/82/83/85 нь middleware-ийн metadata унших бөгөөд
            //  MSE params-ыг тогтмолоор өгдөг тул энд шаардлагагүй.)
            try await espkDriver.selectPublic(0x2003, card: card)
            try await espkDriver.externalKeyAuth(pin: pin, kid: 1, card: card)
            let sig = try await espkDriver.signMessage([UInt8](p), card: card)
            return (Data(sig), p)
        }
    }

    // MARK: - Амьдралын мөчлөг

    /// Нэвтэрсэн/гарсан үед identity-г шинэчилнэ. Гүүр өөрөө нэвтрэлтээс ХАМААРАЛГҮЙ ажилладаг:
    /// ДАН-ий хуудас холбогдож чадаагүй бол "токен суулгаагүй" гэж ойлгодог тул холболтыг үргэлж
    /// хүлээж авч, нэвтрээгүй бол ОЙЛГОМЖТОЙ алдаа буцаах нь дээр.
    func setIdentity(personID: String, certificateB64: String) {
        identity = (personID, certificateB64)
    }

    func clearIdentity() { identity = nil }

    /// Апп асахад дуудна.
    func start() {
        guard !isRunning else { return }
        do {
            let params = NWParameters.tcp
            params.defaultProtocolStack.applicationProtocols
                .insert(NWProtocolWebSocket.Options(), at: 0)
            // ЗӨВХӨН loopback — LAN-д ил гаргахгүй.
            params.requiredLocalEndpoint = .hostPort(host: .ipv4(.loopback), port: .init(rawValue: Self.port)!)
            params.allowLocalEndpointReuse = true
            // (!) `NWListener(using:on:)`-той хамт requiredLocalEndpoint өгвөл EINVAL (POSIX 22)
            // шиднэ. Портыг ЗӨВХӨН requiredLocalEndpoint-оор өгнө — ингэснээр loopback-д л bind
            // хийгдэнэ (`on:` ашиглавал 0.0.0.0 дээр LAN-д ил гарна).
            let l = try NWListener(using: params)
            l.newConnectionHandler = { [weak self] conn in
                Task { @MainActor in self?.accept(conn) }
            }
            l.stateUpdateHandler = { [weak self] state in
                Task { @MainActor in
                    switch state {
                    case .ready: self?.isRunning = true; self?.append("ws://127.0.0.1:\(Self.port) нээгдлээ")
                    case .failed(let e): self?.isRunning = false; self?.append("унтарлаа: \(e.localizedDescription)")
                    case .cancelled: self?.isRunning = false; self?.append("хаагдлаа")
                    case .waiting(let e): self?.append("хүлээж байна: \(e.localizedDescription)")
                    default: break
                    }
                }
            }
            listener = l
            l.start(queue: .main)
        } catch {
            append("эхлүүлж чадсангүй: \(error.localizedDescription)")
        }
    }

    func stop() {
        listener?.cancel()
        listener = nil
        isRunning = false
        pending = nil
    }

    // MARK: - Холболт

    private func accept(_ conn: NWConnection) {
        conn.start(queue: .main)
        receive(on: conn)
    }

    private func receive(on conn: NWConnection) {
        conn.receiveMessage { [weak self] data, _, _, error in
            guard let self else { return }
            if error != nil { conn.cancel(); return }
            guard let data, !data.isEmpty else { conn.cancel(); return }
            Task { @MainActor in
                let reply = await self.handle(data)
                self.send(reply, on: conn)
                self.receive(on: conn)   // ДАН нэг холболтоор олон хүсэлт явуулж болно
            }
        }
    }

    private func send(_ json: String, on conn: NWConnection) {
        let meta = NWProtocolWebSocket.Metadata(opcode: .text)
        let ctx = NWConnection.ContentContext(identifier: "esign", metadata: [meta])
        conn.send(content: Data(json.utf8), contentContext: ctx, isComplete: true,
                  completion: .contentProcessed { _ in })
    }

    // MARK: - Протокол

    private func handle(_ raw: Data) async -> String {
        do {
            guard let root = try JSONSerialization.jsonObject(with: raw) as? [String: Any] else {
                return Self.error("Хоосон эсвэл буруу JSON.")
            }
            let type = root["type"] as? String
            guard type == Self.expectedType else { return Self.error("Танигдахгүй type: \(type ?? "—")") }
            guard let data = root["data"] as? [String: Any],
                  let ott = data["_ott"] as? String, !ott.isEmpty else {
                return Self.error("_ott алга.")
            }
            guard let serverCertB64 = root["cert"] as? String,
                  let serverCertDER = Data(base64Encoded: serverCertB64) else {
                return Self.error("cert (серверийн гэрчилгээ) алга.")
            }
            guard let identity else {
                return Self.error("Нэвтрээгүй байна — eID Mongolia апп-даа нэвтэрнэ үү.")
            }
            let regNum = data["reg-num"] as? String ?? ""
            append("хүсэлт: reg-num=\(regNum) ott=\(ott.prefix(24))…")

            // P.data — хүсэлтийн `data` объектийн indented JSON string. Сервер үүнийг JSON
            // гэж уншдаг тул түлхүүрийн дараалал ач холбогдолгүй; _ott-г эхэнд тавина.
            var keys = ["_ott"]
            if data["reg-num"] != nil { keys.append("reg-num") }
            keys += data.keys.filter { !keys.contains($0) }.sorted()
            let values = data.compactMapValues { $0 as? String }
            let dataJSON = EsignCrypto.indentedJSON(keys: keys.filter { values[$0] != nil }, values: values)

            // Токен эсвэл утас — Settings-ийн toggle + токен залгаастай эсэхээр шийднэ.
            let wantToken = UserDefaults.standard.bool(forKey: AppState.esignUseTokenKey)
            let hasToken = GeregeTokenKit.TokenManager.shared.hasToken()
            let useToken = wantToken && hasToken
            oslog.notice("ESIGN handle: wantToken=\(wantToken, privacy: .public) hasToken=\(hasToken, privacy: .public) reg=\(regNum, privacy: .public)")

            let payload: Data
            let signature: Data
            let algorithm: String
            if useToken {
                // Токен зам — P-г токены ESPK cert-ээр угсарч, локал PIN-ээр PSO зурна.
                let signed = try await signViaToken(dataJSON: dataJSON, regNum: regNum)
                payload = signed.payload
                signature = signed.signature
                algorithm = "RSA-SHA256-PKCS1 (USB токен)"
            } else {
                // Утас зам — нэвтэрсэн хүний cert + threshold PIN2. Гэрчилгээ кэшлэгдээгүй бол
                // "нэвтрээгүй"-ээс ЯЛГАЖ хэлнэ (эс бөгөөс худал зөвлөгөө авч гацна).
                guard !identity.certificateB64.isEmpty,
                      let userCertDER = Data(base64Encoded: identity.certificateB64) else {
                    return Self.error("Гэрчилгээ татагдаагүй байна — гарч аад дахин нэвтэрнэ үү.")
                }
                let p = EsignCrypto.payload(
                    dataJSON: dataJSON,
                    certificateB64: identity.certificateB64,
                    sn: EsignCrypto.certificateSerial(userCertDER),
                    keyIDB64: EsignCrypto.certificateKeyID(userCertDER).base64EncodedString())
                let signed = try await EsignSigner.sign(
                    digest: EsignCrypto.sha256(p),
                    personID: identity.personID,
                    displayText: "Төрийн үйлчилгээнд цахим гарын үсгээр нэвтрэх",
                    onVerificationCode: { [weak self] vc in
                        Task { @MainActor in self?.pending = Pending(verificationCode: vc, regNum: regNum) }
                    })
                pending = nil
                payload = p
                signature = signed.signature
                algorithm = signed.algorithm
            }

            let sealed = try EsignCrypto.seal(payload: payload, serverCertDER: serverCertDER)
            oslog.notice("ESIGN success: algo=\(algorithm, privacy: .public) sig=\(signature.count) P=\(payload.count) cipher=\(sealed.cipherB64.count)")
            append("гарын үсэг амжилттай (\(algorithm))")
            return Self.json([
                "status": "success",
                "signature": signature.base64EncodedString(),
                "cipher": sealed.cipherB64,
                "meta": sealed.metaB64,
            ])
        } catch {
            pending = nil
            pinRequest = nil
            oslog.error("ESIGN error: \(error.localizedDescription, privacy: .public)")
            append("АЛДАА: \(error.localizedDescription)")
            return Self.error(error.localizedDescription)
        }
    }

    // MARK: - Туслах

    private func append(_ line: String) {
        print("[esign] \(line)")   // Windows гүүр шиг console лог — гацаа оношлоход
        log.insert(line, at: 0)
        if log.count > 50 { log.removeLast(log.count - 50) }
    }

    private static func error(_ message: String) -> String {
        json(["status": "error", "message": message])
    }

    private static func json(_ fields: [String: String]) -> String {
        let body = fields.map { "\"\($0.key)\":\"\(EsignCrypto.escape($0.value))\"" }.joined(separator: ",")
        return "{\(body)}"
    }
}
