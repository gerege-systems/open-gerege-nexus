import Foundation

public struct CertificateInfo: Sendable, Equatable {
    public let subject: String
    public let issuer: String
    public let serialNumberHex: String
    public let notBefore: Date
    public let notAfter: Date
    public let signatureAlgorithm: String
    public let publicKeyAlgorithm: String
    public let publicKeyBits: Int
    public let spkiSha256Base64: String
    public let thumbprintSha1Hex: String
    public let thumbprintSha256Hex: String
    public let keyUsages: [String]
    public let extendedKeyUsages: [String]
    public let subjectAlternativeNames: [String]
    public let pemSource: String

    public init(subject: String, issuer: String, serialNumberHex: String,
                notBefore: Date, notAfter: Date, signatureAlgorithm: String,
                publicKeyAlgorithm: String, publicKeyBits: Int,
                spkiSha256Base64: String, thumbprintSha1Hex: String,
                thumbprintSha256Hex: String, keyUsages: [String],
                extendedKeyUsages: [String], subjectAlternativeNames: [String],
                pemSource: String) {
        self.subject = subject
        self.issuer = issuer
        self.serialNumberHex = serialNumberHex
        self.notBefore = notBefore
        self.notAfter = notAfter
        self.signatureAlgorithm = signatureAlgorithm
        self.publicKeyAlgorithm = publicKeyAlgorithm
        self.publicKeyBits = publicKeyBits
        self.spkiSha256Base64 = spkiSha256Base64
        self.thumbprintSha1Hex = thumbprintSha1Hex
        self.thumbprintSha256Hex = thumbprintSha256Hex
        self.keyUsages = keyUsages
        self.extendedKeyUsages = extendedKeyUsages
        self.subjectAlternativeNames = subjectAlternativeNames
        self.pemSource = pemSource
    }

    public func isValid(at now: Date) -> Bool { now >= notBefore && now <= notAfter }
    public func isExpired(at now: Date) -> Bool { now > notAfter }
    public func isNotYetValid(at now: Date) -> Bool { now < notBefore }
    public func remainingValidity(at now: Date) -> TimeInterval? {
        now > notAfter ? nil : notAfter.timeIntervalSince(now)
    }
}
