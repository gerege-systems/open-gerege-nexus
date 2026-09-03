import CommonCrypto
import CryptoKit
import Foundation

/// ESIGN `ENCRYPTED_DATA_SIGN` (`bb4702f31917793f`) схемийн крипто хэсэг — Windows гүүрийн
/// `EsignHybrid.cs`-ийн порт (ESIGNClient.exe-ээс задлан авсан эх алгоритм):
///
///   P         = UTF8( compact JSON { "data": <dataJson STRING>, "certificate": <b64 DER>,
///                                     "sn": <серийн>, "keyID": <b64> } )
///   signature = SHA256(P) дээрх гарын үсэг — утасны threshold түлхүүрээр. Алгоритм нь
///               иргэний БҮРТГЭЛИЙН СХЕМЭЭС хамаарна: SplitKey бүртгэлтэй бол RSA-SHA256-PKCS1
///               (физик ESIGN токентой ЯГ ИЖИЛ), ECDSA бүртгэлтэй бол ECDSA-SHA256.
///   K = random 16B (AES-128), IV = random 16B
///   cipher    = AES-128-CBC/PKCS7(P, K, IV)
///   meta      = RSA-PKCS1v1.5( IV ‖ K ) — ХҮСЭЛТЭД ирсэн серверийн гэрчилгээний public key
///
/// Сервер cipher-ийг тайлаад P-г JSON гэж уншдаг тул талбарын дараалал/зай хамаагүй —
/// чухал нь **гарын үсэг ба шифрлэлт ЯГ ИЖИЛ P байт** дээр хийгдэх явдал. Тиймээс P-г энд
/// гараар угсарч (JSONSerialization дараалал баталгаажуулдаггүй) НЭГ л удаа байт болгоно.
enum EsignCrypto {

    enum Failure: LocalizedError {
        case badServerCert
        case rsaEncrypt(String)

        var errorDescription: String? {
            switch self {
            case .badServerCert: return "Серверийн гэрчилгээ уншигдсангүй."
            case .rsaEncrypt(let m): return "meta шифрлэх амжилтгүй: \(m)"
            }
        }
    }

    // MARK: - P угсралт

    /// Хүсэлтийн `data` объектийг Newtonsoft `JObject.ToString()`-той ойролцоо indented JSON
    /// болгоно (2 зай, CRLF, `": "`, юникодыг литералаар). Сервер үүнийг JSON гэж уншина.
    /// Түлхүүрийн дараалал нь хүсэлтэд ирсэн эрэмбийг хадгална.
    static func indentedJSON(keys: [String], values: [String: String]) -> String {
        var out = "{\r\n"
        for (i, key) in keys.enumerated() {
            out += "  \"\(escape(key))\": \"\(escape(values[key] ?? ""))\""
            if i < keys.count - 1 { out += "," }
            out += "\r\n"
        }
        return out + "}"
    }

    /// P — гарын үсэг ба шифрлэлтийн ЕРӨНХИЙ эх байт. Талбарын дараалал C#-тай ижил.
    static func payload(dataJSON: String, certificateB64: String, sn: String, keyIDB64: String) -> Data {
        let json = "{\"data\":\"\(escape(dataJSON))\"," +
                   "\"certificate\":\"\(escape(certificateB64))\"," +
                   "\"sn\":\"\(escape(sn))\"," +
                   "\"keyID\":\"\(escape(keyIDB64))\"}"
        return Data(json.utf8)
    }

    /// JSON мөрийн escape — Newtonsoft шиг юникодыг литералаар үлдээнэ.
    static func escape(_ s: String) -> String {
        var out = ""
        out.reserveCapacity(s.count + 8)
        for ch in s.unicodeScalars {
            switch ch {
            case "\"": out += "\\\""
            case "\\": out += "\\\\"
            case "\u{08}": out += "\\b"
            case "\u{0C}": out += "\\f"
            case "\n": out += "\\n"
            case "\r": out += "\\r"
            case "\t": out += "\\t"
            default:
                if ch.value < 0x20 { out += String(format: "\\u%04x", ch.value) } else { out.unicodeScalars.append(ch) }
            }
        }
        return out
    }

    // MARK: - Hybrid шифрлэлт

    struct Sealed {
        let cipherB64: String
        let metaB64: String
    }

    /// `cipher` + `meta`-г үүсгэнэ. K/IV нь дуудалт бүрд шинэ санамсаргүй утга.
    static func seal(payload p: Data, serverCertDER: Data) throws -> Sealed {
        var key = Data(count: 16)   // AES-128 — ESIGN-ий CKA_VALUE_LEN=16
        var iv = Data(count: 16)
        try randomFill(&key)
        try randomFill(&iv)

        let cipher = try aesCBCEncrypt(p, key: key, iv: iv)
        let meta = try rsaPKCS1Encrypt(iv + key, certDER: serverCertDER)   // IV ЭХЭНД
        return Sealed(cipherB64: cipher.base64EncodedString(), metaB64: meta.base64EncodedString())
    }

    static func sha256(_ d: Data) -> Data { Data(SHA256.hash(data: d)) }

    // MARK: - Primitive-ууд

    private static func randomFill(_ d: inout Data) throws {
        let n = d.count
        let ok = d.withUnsafeMutableBytes { SecRandomCopyBytes(kSecRandomDefault, n, $0.baseAddress!) }
        guard ok == errSecSuccess else { throw Failure.rsaEncrypt("санамсаргүй байт үүсгэж чадсангүй") }
    }

    /// AES-128-CBC/PKCS7. CryptoKit-д CBC байхгүй тул CommonCrypto.
    static func aesCBCEncrypt(_ plain: Data, key: Data, iv: Data) throws -> Data {
        var out = Data(count: plain.count + kCCBlockSizeAES128)
        let outCapacity = out.count   // closure дотроос `out`-д хандвал exclusivity зөрчигдөнө
        var moved = 0
        let status = out.withUnsafeMutableBytes { o in
            plain.withUnsafeBytes { p in
                key.withUnsafeBytes { k in
                    iv.withUnsafeBytes { v in
                        CCCrypt(CCOperation(kCCEncrypt), CCAlgorithm(kCCAlgorithmAES),
                                CCOptions(kCCOptionPKCS7Padding),
                                k.baseAddress, key.count, v.baseAddress,
                                p.baseAddress, plain.count,
                                o.baseAddress, outCapacity, &moved)
                    }
                }
            }
        }
        guard status == kCCSuccess else { throw Failure.rsaEncrypt("AES алдаа \(status)") }
        return out.prefix(moved)
    }

    /// RSA PKCS#1 v1.5 encrypt — серверийн гэрчилгээний public key-ээр.
    static func rsaPKCS1Encrypt(_ plain: Data, certDER: Data) throws -> Data {
        guard let cert = SecCertificateCreateWithData(nil, certDER as CFData),
              let pub = SecCertificateCopyKey(cert) else { throw Failure.badServerCert }
        return try rsaPKCS1Encrypt(plain, publicKey: pub)
    }

    static func rsaPKCS1Encrypt(_ plain: Data, publicKey: SecKey) throws -> Data {
        var err: Unmanaged<CFError>?
        guard let out = SecKeyCreateEncryptedData(publicKey, .rsaEncryptionPKCS1, plain as CFData, &err) else {
            throw Failure.rsaEncrypt(err?.takeRetainedValue().localizedDescription ?? "тодорхойгүй")
        }
        return out as Data
    }

    static func aesCBCDecrypt(_ cipher: Data, key: Data, iv: Data) throws -> Data {
        var out = Data(count: cipher.count + kCCBlockSizeAES128)
        let outCapacity = out.count
        var moved = 0
        let status = out.withUnsafeMutableBytes { o in
            cipher.withUnsafeBytes { c in
                key.withUnsafeBytes { k in
                    iv.withUnsafeBytes { v in
                        CCCrypt(CCOperation(kCCDecrypt), CCAlgorithm(kCCAlgorithmAES),
                                CCOptions(kCCOptionPKCS7Padding),
                                k.baseAddress, key.count, v.baseAddress,
                                c.baseAddress, cipher.count,
                                o.baseAddress, outCapacity, &moved)
                    }
                }
            }
        }
        guard status == kCCSuccess else { throw Failure.rsaEncrypt("AES decrypt алдаа \(status)") }
        return out.prefix(moved)
    }

    // MARK: - Өөрөө шалгах (Windows гүүрийн `selftest`-ийн эквивалент)

    /// "Сервер"-ийн үүрэг гүйцэтгэж бүтэн round-trip хийнэ: RSA түлхүүр үүсгэх → seal →
    /// meta-г тайлж IV‖K сэргээх → cipher-ийг тайлж P-г буцааж авах → эхтэй тулгах.
    /// Гарын үсгийн хэсэг ОРООГҮЙ (утас шаардана) — энэ нь зөвхөн крипто давхаргын шалгалт.
    static func selfTest() -> String {
        do {
            let attrs: [String: Any] = [
                kSecAttrKeyType as String: kSecAttrKeyTypeRSA,
                kSecAttrKeySizeInBits as String: 2048,
            ]
            var err: Unmanaged<CFError>?
            guard let priv = SecKeyCreateRandomKey(attrs as CFDictionary, &err),
                  let pub = SecKeyCopyPublicKey(priv) else {
                return "✗ RSA түлхүүр үүсгэж чадсангүй"
            }
            let dataJSON = indentedJSON(keys: ["_ott", "reg-num"],
                                        values: ["_ott": UUID().uuidString, "reg-num": "ФА92040910"])
            let p = payload(dataJSON: dataJSON, certificateB64: "TUlJQg==", sn: "0A1B", keyIDB64: "a2V5")

            var key = Data(count: 16), iv = Data(count: 16)
            try randomFill(&key); try randomFill(&iv)
            let cipher = try aesCBCEncrypt(p, key: key, iv: iv)
            let meta = try rsaPKCS1Encrypt(iv + key, publicKey: pub)

            // ── сервер тал ──
            guard let blobCF = SecKeyCreateDecryptedData(priv, .rsaEncryptionPKCS1, meta as CFData, &err) else {
                return "✗ meta тайлагдсангүй"
            }
            let blob = blobCF as Data
            guard blob.count == 32 else { return "✗ IV‖K урт буруу: \(blob.count) (32 байх ёстой)" }
            let back = try aesCBCDecrypt(cipher, key: blob.suffix(16), iv: blob.prefix(16))
            guard back == p else { return "✗ P тулгалт зөрлөө" }
            guard let pj = try JSONSerialization.jsonObject(with: back) as? [String: Any],
                  let inner = pj["data"] as? String,
                  let innerObj = try JSONSerialization.jsonObject(with: Data(inner.utf8)) as? [String: Any],
                  innerObj["_ott"] != nil else {
                return "✗ P нь зөв JSON биш (эсвэл data доторх _ott алга)"
            }
            return "✓ Крипто зөв: meta→IV‖K→cipher→P бүрэн сэргээгдлээ (P \(p.count)B, cipher \(cipher.count)B, meta \(meta.count)B)"
        } catch {
            return "✗ \(error.localizedDescription)"
        }
    }

    /// `sn` — физик токен дээр төхөөрөмжийн серийн дугаар байдаг. Программ токенд ийм зүйл
    /// байхгүй тул хэрэглэгчийн гэрчилгээний серийн дугаарыг ашиглана.
    ///
    /// Windows клиентийн `EsignCertParser.Serial` нь `X509Certificate2.SerialNumber`-ийг өгдөг —
    /// тэр нь ASN.1 INTEGER-ийг задалсан ТОО тул эерэг тэмдгийн тэргүүлэх `00` padding БАЙХГҮЙ.
    /// `SecCertificateCopySerialNumberData` нь DER-ийн агуулгыг байтаар нь өгдөг тул padding-ыг
    /// энд хасна — эс бөгөөс ижил гэрчилгээнд хоёр клиент өөр `sn` явуулна.
    static func certificateSerial(_ certDER: Data) -> String {
        guard let cert = SecCertificateCreateWithData(nil, certDER as CFData),
              let sn = SecCertificateCopySerialNumberData(cert, nil) as Data? else { return "" }
        var bytes = Array(sn)
        while bytes.count > 1 && bytes[0] == 0 { bytes.removeFirst() }
        return bytes.map { String(format: "%02X", $0) }.joined()
    }

    /// `keyID` — PKCS#11 токены CKA_ID-тэй дүйцэх утга. Windows клиенттэй (`EsignCertParser.KeyId`)
    /// ЯГ ИЖИЛ дүрэм: гэрчилгээний SubjectKeyIdentifier өргөтгөл байвал түүнийг, үгүй бол public
    /// key-ийн SHA-1. Хоёр клиент ижил гэрчилгээнд ижил keyID явуулах ёстой.
    static func certificateKeyID(_ certDER: Data) -> Data {
        guard let cert = SecCertificateCreateWithData(nil, certDER as CFData) else { return Data() }
        if let values = SecCertificateCopyValues(cert, [kSecOIDSubjectKeyIdentifier] as CFArray, nil)
            as? [String: Any],
           let entry = values[kSecOIDSubjectKeyIdentifier as String] as? [String: Any],
           let ski = skiBytes(entry["value"]) {
            return ski
        }
        // Fallback: public key-ийн SHA-1 (.NET-ийн `SHA1.HashData(x.GetPublicKey())`-тэй дүйцнэ —
        // хоёр тал ч SubjectPublicKeyInfo-гийн ТҮЛХҮҮРИЙН байтыг хэшлэнэ, SPKI бүхэлд нь биш).
        guard let key = SecCertificateCopyKey(cert),
              let raw = SecKeyCopyExternalRepresentation(key, nil) as Data? else { return Data() }
        return Data(Insecure.SHA1.hash(data: raw))
    }

    /// SecCertificateCopyValues нь SKI-г Data эсвэл хэсэгчилсэн бүтэц болгон өгч болно.
    private static func skiBytes(_ value: Any?) -> Data? {
        if let d = value as? Data, !d.isEmpty { return d }
        if let arr = value as? [[String: Any]] {
            for item in arr { if let d = item["value"] as? Data, !d.isEmpty { return d } }
        }
        return nil
    }
}
