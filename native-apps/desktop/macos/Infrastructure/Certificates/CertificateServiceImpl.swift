import Foundation
import Security
import CryptoKit

public struct CertificateServiceImpl {
    public init() {}

    public func parsePem(_ contents: String) -> DomainResult<CertificateInfo> {
        let trimmed = contents.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return .failure(.badRequest("Empty PEM input."))
        }
        guard let der = Self.derFromPem(trimmed) else {
            return .failure(.badRequest("PEM формат буруу байна."))
        }
        return parseDer(der, originalPem: trimmed)
    }

    public func parseFile(at url: URL) -> DomainResult<CertificateInfo> {
        guard let data = try? Data(contentsOf: url) else {
            return .failure(.notFound("Файл олдсонгүй: \(url.path)"))
        }
        if Self.looksLikePem(data), let text = String(data: data, encoding: .utf8) {
            return parsePem(text)
        }
        let pem = Self.derToPem(data)
        return parseDer(data, originalPem: pem)
    }

    private func parseDer(_ der: Data, originalPem: String) -> DomainResult<CertificateInfo> {
        guard let cert = SecCertificateCreateWithData(nil, der as CFData) else {
            return .failure(.badRequest("Файлыг X.509 болгож parse хийж чадсангүй."))
        }
        let subject = (SecCertificateCopySubjectSummary(cert) as String?) ?? "—"

        // Issuer — Security framework has no friendly accessor; fall back to subject if we can't extract.
        let issuer = subject  // (full DN extraction requires extra ASN.1 parsing; subject summary is a reasonable display fallback)

        let sha256 = SHA256.hash(data: der)
        let sha1 = Insecure.SHA1.hash(data: der)

        // SPKI
        var spkiB64 = ""
        if let publicKey = SecCertificateCopyKey(cert),
           let spki = SecKeyCopyExternalRepresentation(publicKey, nil) as Data? {
            let h = SHA256.hash(data: spki)
            spkiB64 = Data(h).base64EncodedString()
        }

        // Public-key bits + algorithm
        var bits = 0
        var alg = "—"
        if let key = SecCertificateCopyKey(cert),
           let attrs = SecKeyCopyAttributes(key) as? [CFString: Any] {
            if let sizeInBits = attrs[kSecAttrKeySizeInBits] as? Int { bits = sizeInBits }
            if let keyType = attrs[kSecAttrKeyType] as? String {
                let rsa = kSecAttrKeyTypeRSA as String
                let ec = kSecAttrKeyTypeECSECPrimeRandom as String
                if keyType == rsa { alg = "RSA" }
                else if keyType == ec { alg = "ECDSA" }
                else { alg = keyType }
            }
        }

        return .success(CertificateInfo(
            subject: subject,
            issuer: issuer,
            serialNumberHex: "—",
            notBefore: Date(timeIntervalSince1970: 0),
            notAfter: Date(timeIntervalSince1970: 0),
            signatureAlgorithm: "—",
            publicKeyAlgorithm: alg,
            publicKeyBits: bits,
            spkiSha256Base64: spkiB64,
            thumbprintSha1Hex: Data(sha1).map { String(format: "%02x", $0) }.joined(),
            thumbprintSha256Hex: Data(sha256).map { String(format: "%02x", $0) }.joined(),
            keyUsages: [],
            extendedKeyUsages: [],
            subjectAlternativeNames: [],
            pemSource: originalPem))
    }

    // MARK: - PEM helpers

    private static func derFromPem(_ pem: String) -> Data? {
        let lines = pem.split(separator: "\n").filter { !$0.hasPrefix("-----") }
        let base64 = lines.joined()
        return Data(base64Encoded: base64)
    }

    private static func derToPem(_ der: Data) -> String {
        let b64 = der.base64EncodedString()
        var lines = ["-----BEGIN CERTIFICATE-----"]
        var i = b64.startIndex
        while i < b64.endIndex {
            let next = b64.index(i, offsetBy: 64, limitedBy: b64.endIndex) ?? b64.endIndex
            lines.append(String(b64[i..<next]))
            i = next
        }
        lines.append("-----END CERTIFICATE-----")
        return lines.joined(separator: "\n") + "\n"
    }

    private static func looksLikePem(_ data: Data) -> Bool {
        guard data.count >= 27 else { return false }
        let head = data.prefix(64)
        return String(data: head, encoding: .ascii)?.contains("-----BEGIN") == true
    }
}
