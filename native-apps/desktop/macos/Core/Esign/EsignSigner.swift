import Foundation

/// ESIGN "программ токен"-ий **backend тал** — физик USB токенгүйгээр ДАН/isf.mn-д тоон
/// гарын үсгээр нэвтрэхэд хэрэгтэй хоёр primitive:
///
///   1. Иргэний гэрчилгээ (base64 DER) — ДАН-д илгээх payload-д хавсаргана. Гарын үсэг нь
///      гэрчилгээг АГУУЛСАН payload дээр тавигддаг тул зурахаас ӨМНӨ мэдэгдсэн байх ёстой →
///      нэвтэрсэн даруйд `AppState.loadPersonExtras` татаж Keychain-д кэшилдэг.
///   2. ДУРЫН 32 байт digest-ийг утасны PIN2 түлхүүрээр зуруулж ТҮҮХИЙ гарын үсэг авах
///      (`/api/esign-sign` → `/api/status`). PDF stamp хийхгүй.
///
/// (!) ws гүүр (`127.0.0.1:59001` дээрх `ENCRYPTED_DATA_SIGN`), P payload-ийн БАЙТ формат,
/// AES cipher + RSA `meta` нь энд БАЙХГҮЙ — ДАН-ий протоколын дугтуй болон серверийн RSA
/// гэрчилгээ шаардлагатай (Windows апп-ын `Esign*.cs`). Тэдгээр ирэхэд энэ файлын дээр
/// нэмэгдэнэ; доорх хэсэг нь дугтуйнаас ХАМААРАЛГҮЙ тул одооноос эцсийн хэлбэртэй.
enum EsignSigner {

    enum Failure: LocalizedError {
        case noCertificate
        case notSigned(String)

        var errorDescription: String? {
            switch self {
            case .noCertificate:
                return "Гэрчилгээ олдсонгүй — дахин нэвтэрнэ үү."
            case .notSigned(let why):
                return "Гарын үсэг зурагдсангүй: \(why)"
            }
        }
    }

    /// Зурах гэрчилгээ — `signing` (PIN2) давуу, эс бөгөөс `auth`. ДАН аль гэрчилгээг
    /// хүлээн авах нь тодруулагдаагүй тул сонголтыг НЭГ л энд төвлөрүүлэв.
    @MainActor
    static func certificateB64(_ state: AppState) throws -> String {
        let cert = state.esignSigningCertB64.isEmpty ? state.esignAuthCertB64 : state.esignSigningCertB64
        guard !cert.isEmpty else { throw Failure.noCertificate }
        return cert
    }

    /// Гарын үсгийн үр дүн — ДАН-ий хариунд шаардагдах талбарууд.
    struct Signed {
        /// Түүхий гарын үсгийн байт. SplitKey бүртгэлтэй иргэнд RSA-SHA256-PKCS1 (тогтмол
        /// урт, RSA-4096 бол 512 байт), ECDSA бүртгэлтэйд ECDSA-SHA256 (ASN.1 DER).
        let signature: Data
        /// Гарын үсэг зурсан гэрчилгээ (base64 DER) — сервер юугаар зурснаа буцаана.
        let certificateB64: String
        let algorithm: String
    }

    /// 32 байт digest-ийг утсаар зуруулна: PIN2 push → COMPLETE болтол poll → түүхий гарын үсэг.
    /// `vc` (баталгаажуулах код) нь push илгээмэгц `onVerificationCode`-оор гарна — хэрэглэгч
    /// утсан дээрхтэй тулгах ёстой тул дэлгэцэнд ЗААВАЛ харуулна.
    static func sign(
        digest: Data,
        personID: String,
        displayText: String,
        onVerificationCode: @Sendable (String) -> Void = { _ in }
    ) async throws -> Signed {
        precondition(digest.count == 32, "ESIGN digest нь SHA-256 (32 байт) байх ёстой")
        let started: EsignSignResponse = try await APIClient.shared.request(
            .esignSign(personID: personID,
                       digestB64: digest.base64EncodedString(),
                       displayText: displayText))
        if let vc = started.vc, !vc.isEmpty { onVerificationCode(vc) }

        let status = try await APIClient.shared.waitForAuth(
            sessionID: started.sessionId, pollToken: started.pollToken ?? "")
        guard status.isOK else { throw Failure.notSigned(status.endResult ?? "UNKNOWN") }
        guard let sigB64 = status.signatureValueB64,
              let signature = Data(base64Encoded: sigB64), !signature.isEmpty else {
            throw Failure.notSigned("гарын үсгийн утга ирсэнгүй")
        }
        return Signed(
            signature: signature,
            // Сервер юугаар зурснаа буцаадаг — кэшлэсэн гэрчилгээтэй зөрвөл СЕРВЕРИЙНХ зөв.
            certificateB64: status.certificateDerB64 ?? "",
            // Алгоритмыг СЕРВЕР хэлнэ — схемээс хамаарна, клиент таамаглахгүй.
            algorithm: status.signatureAlgorithm ?? "")
    }
}
