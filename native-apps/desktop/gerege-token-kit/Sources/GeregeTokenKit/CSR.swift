import Foundation
import CommonCrypto

/// PKCS#10 Certification Request (CSR) builder.
/// EC P-256 + ECDSA-SHA256 дэмжинэ — CA руу илгээж сертификат авна.
///
/// RFC 2986 + RFC 5480 (EC) + RFC 5758 (ECDSA-SHA256).
///
/// Signer closure нь public key-ийн тохирох private key-ээр
/// `tbsCertificationRequestInfo` DER-ийн SHA-256 hash-г зурна.
/// (PKCS#11 session эсвэл BioPassDriver-ийн signECDSA дуудна)
public enum CSR {

    // MARK: - OIDs (DER-encoded)

    // 1.2.840.10045.2.1 — id-ecPublicKey
    static let oidEcPublicKey: [UInt8] = [
        0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x02, 0x01
    ]
    // 1.2.840.10045.3.1.7 — secp256r1 (P-256)
    static let oidP256: [UInt8] = [
        0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07
    ]
    // 1.2.840.10045.4.3.2 — ecdsa-with-SHA256
    static let oidEcdsaWithSHA256: [UInt8] = [
        0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x04, 0x03, 0x02
    ]
    // 2.5.4.3 — commonName
    static let oidCN: [UInt8] = [0x55, 0x04, 0x03]
    // 2.5.4.10 — organizationName
    static let oidO: [UInt8] = [0x55, 0x04, 0x0A]
    // 2.5.4.6 — countryName
    static let oidC: [UInt8] = [0x55, 0x04, 0x06]
    // 1.2.840.113549.1.9.1 — emailAddress
    static let oidEmail: [UInt8] = [
        0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x09, 0x01
    ]

    // MARK: - Public API

    public struct Subject {
        public var commonName: String
        public var organization: String?
        public var country: String?
        public var email: String?

        public init(commonName: String, organization: String? = nil,
                    country: String? = nil, email: String? = nil) {
            self.commonName = commonName
            self.organization = organization
            self.country = country
            self.email = email
        }
    }

    /// EC P-256 уламжлалт pubkey (65 байт, `04 || X || Y`) болон signer closure-аас
    /// PKCS#10 CSR DER + PEM буцаана.
    /// - Parameters:
    ///   - subject: Subject нэр (CN заавал)
    ///   - publicKeyPoint: Uncompressed EC point (65 байт, эхлэх `0x04`)
    ///   - signer: SHA-256 hash-г авч ECDSA signature (r||s 64 байт эсвэл DER ECDSA-Sig) буцаах closure
    public static func buildP256(
        subject: Subject,
        publicKeyPoint: [UInt8],
        signer: ([UInt8]) async throws -> [UInt8]
    ) async throws -> (der: [UInt8], pem: String) {
        guard publicKeyPoint.count == 65, publicKeyPoint[0] == 0x04 else {
            throw CSRError.invalidPublicKey
        }

        // 1. CertificationRequestInfo
        let cri = buildCertificationRequestInfo(subject: subject,
                                                 publicKeyPoint: publicKeyPoint)

        // 2. Sign SHA-256(CRI)
        let hash = sha256(cri)
        let rawSig = try await signer(hash)

        // Signer-аас r||s (64 байт) ирсэн бол DER-лэх (ECDSA-Sig-Value)
        let derSig: [UInt8]
        if rawSig.count == 64 {
            derSig = encodeECDSASigDER(rs: rawSig)
        } else {
            derSig = rawSig  // assume DER already
        }

        // 3. signatureAlgorithm = SEQUENCE { OID ecdsa-with-SHA256 }
        let sigAlg = derSequence(derOID(oidEcdsaWithSHA256))

        // 4. Outer CertificationRequest
        let inner = cri + sigAlg + derBitString(derSig)
        let csr = derSequence(inner)

        let pem = pemWrap(csr, label: "CERTIFICATE REQUEST")
        return (csr, pem)
    }

    // MARK: - CertificationRequestInfo builder

    static func buildCertificationRequestInfo(subject: Subject,
                                               publicKeyPoint: [UInt8]) -> [UInt8] {
        // version INTEGER (0)
        let version: [UInt8] = [0x02, 0x01, 0x00]

        // subject Name (RDNSequence)
        var rdns = [UInt8]()
        if let c = subject.country { rdns += rdn(oid: oidC, value: c, asString: .printable) }
        if let o = subject.organization { rdns += rdn(oid: oidO, value: o, asString: .utf8) }
        rdns += rdn(oid: oidCN, value: subject.commonName, asString: .utf8)
        if let e = subject.email { rdns += rdn(oid: oidEmail, value: e, asString: .ia5) }
        let subjectSeq = derSequence(rdns)

        // SubjectPublicKeyInfo
        //   algorithm = SEQUENCE { id-ecPublicKey, secp256r1 }
        let algId = derSequence(derOID(oidEcPublicKey) + derOID(oidP256))
        //   subjectPublicKey BIT STRING = 0x00 || publicKeyPoint
        let spki = derSequence(algId + derBitString(publicKeyPoint))

        // attributes [0] IMPLICIT — empty
        let attrs: [UInt8] = [0xA0, 0x00]

        return derSequence(version + subjectSeq + spki + attrs)
    }

    // MARK: - DER primitives

    enum StringEncoding { case utf8, printable, ia5 }

    static func rdn(oid: [UInt8], value: String, asString kind: StringEncoding) -> [UInt8] {
        let tag: UInt8
        switch kind {
        case .utf8: tag = 0x0C          // UTF8String
        case .printable: tag = 0x13     // PrintableString
        case .ia5: tag = 0x16           // IA5String
        }
        let bytes = Array(value.utf8)
        let strTLV: [UInt8] = [tag] + derLength(bytes.count) + bytes
        // AttributeTypeAndValue ::= SEQUENCE { type, value }
        let atv = derSequence(derOID(oid) + strTLV)
        // RDN ::= SET OF ATV
        return [0x31] + derLength(atv.count) + atv
    }

    static func derOID(_ bytes: [UInt8]) -> [UInt8] {
        [0x06] + derLength(bytes.count) + bytes
    }

    static func derSequence(_ content: [UInt8]) -> [UInt8] {
        [0x30] + derLength(content.count) + content
    }

    /// BIT STRING: [0x03][len][0x00 unused bits][bytes]
    static func derBitString(_ bytes: [UInt8]) -> [UInt8] {
        let body: [UInt8] = [0x00] + bytes
        return [0x03] + derLength(body.count) + body
    }

    static func derLength(_ n: Int) -> [UInt8] {
        if n < 0x80 { return [UInt8(n)] }
        if n <= 0xFF { return [0x81, UInt8(n)] }
        if n <= 0xFFFF { return [0x82, UInt8(n >> 8), UInt8(n & 0xFF)] }
        return [0x83, UInt8(n >> 16), UInt8((n >> 8) & 0xFF), UInt8(n & 0xFF)]
    }

    /// Encode r||s (64 байт) → ECDSA-Sig-Value DER: SEQUENCE { r INTEGER, s INTEGER }
    public static func encodeECDSASigDER(rs: [UInt8]) -> [UInt8] {
        let half = rs.count / 2
        let r = Array(rs.prefix(half))
        let s = Array(rs.suffix(half))
        return derSequence(derInteger(r) + derInteger(s))
    }

    /// DER INTEGER — leading zeros-ыг устгаж, хамгийн их bit=1 бол 0x00 prepend.
    static func derInteger(_ bytes: [UInt8]) -> [UInt8] {
        var trimmed = bytes
        while trimmed.count > 1 && trimmed.first == 0x00 { trimmed.removeFirst() }
        if let first = trimmed.first, first & 0x80 != 0 {
            trimmed.insert(0x00, at: 0)
        }
        return [0x02] + derLength(trimmed.count) + trimmed
    }

    // MARK: - PEM wrap

    static func pemWrap(_ der: [UInt8], label: String) -> String {
        let b64 = Data(der).base64EncodedString()
        var lines = [String]()
        var idx = b64.startIndex
        while idx < b64.endIndex {
            let end = b64.index(idx, offsetBy: 64, limitedBy: b64.endIndex) ?? b64.endIndex
            lines.append(String(b64[idx..<end]))
            idx = end
        }
        return "-----BEGIN \(label)-----\n" + lines.joined(separator: "\n") + "\n-----END \(label)-----\n"
    }

    // MARK: - Hash

    static func sha256(_ data: [UInt8]) -> [UInt8] {
        var digest = [UInt8](repeating: 0, count: Int(CC_SHA256_DIGEST_LENGTH))
        CC_SHA256(data, CC_LONG(data.count), &digest)
        return digest
    }
}

public enum CSRError: LocalizedError {
    case invalidPublicKey
    case signerFailed(String)

    public var errorDescription: String? {
        switch self {
        case .invalidPublicKey:    return "Public key буруу (P-256 uncompressed 65 байт байх ёстой)."
        case .signerFailed(let m): return "Signer алдаа: \(m)"
        }
    }
}
