import Foundation

/// Logical provider behind a token.
public enum TokenBackend: String, Sendable {
    case pkcs11 = "Pkcs11"
    case macKeychain = "MacKeychain"
}

public struct TokenInfo: Sendable, Equatable {
    public let id: String
    public let label: String
    public let manufacturer: String
    public let serialNumber: String
    public let model: String
    public let backend: TokenBackend
    public let removable: Bool
    public let hardwareSlot: Bool
    public let isLoggedIn: Bool

    public init(id: String, label: String, manufacturer: String, serialNumber: String,
                model: String, backend: TokenBackend, removable: Bool,
                hardwareSlot: Bool, isLoggedIn: Bool) {
        self.id = id
        self.label = label
        self.manufacturer = manufacturer
        self.serialNumber = serialNumber
        self.model = model
        self.backend = backend
        self.removable = removable
        self.hardwareSlot = hardwareSlot
        self.isLoggedIn = isLoggedIn
    }
}

public enum TokenObjectKind: String, Sendable {
    case privateKey
    case publicKey
    case certificate
    case data
}

public struct TokenObject: Sendable, Equatable {
    public let id: String
    public let label: String?
    public let kind: TokenObjectKind
    public let keyAlgorithm: String
    public let keyBits: Int
    public let canSign: Bool
    public let canDecrypt: Bool

    public init(id: String, label: String?, kind: TokenObjectKind, keyAlgorithm: String,
                keyBits: Int, canSign: Bool, canDecrypt: Bool) {
        self.id = id
        self.label = label
        self.kind = kind
        self.keyAlgorithm = keyAlgorithm
        self.keyBits = keyBits
        self.canSign = canSign
        self.canDecrypt = canDecrypt
    }
}

public enum SigningAlg: String, Sendable {
    case sha256WithRsa = "RSA-SHA256"
    case sha384WithRsa = "RSA-SHA384"
    case sha512WithRsa = "RSA-SHA512"
    case ecdsaSha256 = "ECDSA-SHA256"
    case ecdsaSha384 = "ECDSA-SHA384"
    case ecdsaSha512 = "ECDSA-SHA512"
}

public enum KeyAlgorithm: String, Sendable {
    case rsa = "RSA"
    case ecdsaP256 = "EC-P256"
    case ecdsaP384 = "EC-P384"
    case ecdsaP521 = "EC-P521"
}

public struct KeyParams: Sendable, Equatable {
    public let algorithm: KeyAlgorithm
    public let rsaBits: Int
    public let label: String
    public let sensitive: Bool
    public let extractable: Bool

    public init(algorithm: KeyAlgorithm, rsaBits: Int = 3072,
                label: String = "Gerege device key",
                sensitive: Bool = true, extractable: Bool = false) {
        self.algorithm = algorithm
        self.rsaBits = rsaBits
        self.label = label
        self.sensitive = sensitive
        self.extractable = extractable
    }
}
