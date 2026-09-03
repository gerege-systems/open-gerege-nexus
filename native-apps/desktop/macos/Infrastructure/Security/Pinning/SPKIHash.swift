import Foundation
import CryptoKit
import Security

// SHA-256 of a certificate's SubjectPublicKeyInfo (SPKI) — the value that
// matches `openssl x509 -pubkey | openssl pkey -outform der | sha256`.
//
// Why this exists: `SecKeyCopyExternalRepresentation` returns the *raw* key
// (PKCS#1 modulus/exponent for RSA, X9.63 `04||X||Y` for EC) — not the full
// DER-encoded SubjectPublicKeyInfo. Hashing the raw bytes produces a digest
// different from every standard pinning tool's output, so pin checks silently
// fail. The fix is to prepend the well-known SPKI ASN.1 prefix bytes for the
// (algorithm, key-size) tuple, then hash.
//
// Ported from the iOS client (ios/Core/Network/SPKIHash.swift) — keep the two
// in sync so the desktop + mobile pin sets stay identical.
enum SPKIHash {

    /// SHA-256 (base64) of the cert's full SubjectPublicKeyInfo DER. Returns
    /// nil if the key type is not in the supported prefix table — caller
    /// should treat nil as "cannot pin this cert" and move on (do NOT pass).
    static func sha256Base64(of cert: SecCertificate) -> String? {
        guard let pubKey = SecCertificateCopyKey(cert),
              let raw = SecKeyCopyExternalRepresentation(pubKey, nil) as Data?,
              let attrs = SecKeyCopyAttributes(pubKey) as? [String: Any] else {
            return nil
        }
        let keyType = attrs[kSecAttrKeyType as String] as? String ?? ""
        let keySize = attrs[kSecAttrKeySizeInBits as String] as? Int ?? 0
        guard let prefix = asn1Prefix(keyType: keyType, keySize: keySize) else {
            return nil
        }
        var spki = prefix
        spki.append(raw)
        return Data(SHA256.hash(data: spki)).base64EncodedString()
    }

    /// SPKI ASN.1 prefix bytes that, when prepended to the raw key bytes
    /// returned by `SecKeyCopyExternalRepresentation`, reconstruct the full
    /// `SubjectPublicKeyInfo` DER. Hashing prefix||raw == hashing the SPKI.
    ///
    /// Covers the algorithms used by Let's Encrypt (the issuer for
    /// api.eidmongol.mn / ca.gerege.mn): ECDSA P-256 (typical leaf),
    /// ECDSA P-384 (the E8 intermediate), and RSA-2048/4096 as a fallback.
    private static func asn1Prefix(keyType: String, keySize: Int) -> Data? {
        let ec = kSecAttrKeyTypeECSECPrimeRandom as String
        let rsa = kSecAttrKeyTypeRSA as String
        if keyType == ec, keySize == 256 {
            return Data([
                0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2A, 0x86,
                0x48, 0xCE, 0x3D, 0x02, 0x01, 0x06, 0x08, 0x2A,
                0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07, 0x03,
                0x42, 0x00,
            ])
        }
        if keyType == ec, keySize == 384 {
            return Data([
                0x30, 0x76, 0x30, 0x10, 0x06, 0x07, 0x2A, 0x86,
                0x48, 0xCE, 0x3D, 0x02, 0x01, 0x06, 0x05, 0x2B,
                0x81, 0x04, 0x00, 0x22, 0x03, 0x62, 0x00,
            ])
        }
        if keyType == rsa, keySize == 2048 {
            return Data([
                0x30, 0x82, 0x01, 0x22, 0x30, 0x0D, 0x06, 0x09,
                0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01,
                0x01, 0x05, 0x00, 0x03, 0x82, 0x01, 0x0F, 0x00,
            ])
        }
        if keyType == rsa, keySize == 4096 {
            return Data([
                0x30, 0x82, 0x02, 0x22, 0x30, 0x0D, 0x06, 0x09,
                0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01,
                0x01, 0x05, 0x00, 0x03, 0x82, 0x02, 0x0F, 0x00,
            ])
        }
        return nil
    }
}
