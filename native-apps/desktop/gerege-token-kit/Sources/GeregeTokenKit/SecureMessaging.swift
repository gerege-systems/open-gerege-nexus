import Foundation
import CommonCrypto

/// ePass2003 Secure Messaging — mutual auth + SM APDU wrapping.
/// OpenSC card-epass2003.c-ийн Swift port.
///
/// Protocol: SCP01 variant
/// Non-FIPS: 3DES-CBC + retail MAC
/// FIPS: AES-128-CBC + AES-CBC-MAC
public final class SecureMessaging {

    /// Session keys (mutual auth дараа тогтоогдоно)
    public private(set) var sessionKeyEnc: [UInt8] = []
    public private(set) var sessionKeyMac: [UInt8] = []
    public private(set) var icvMac: [UInt8] = Array(repeating: 0, count: 16)
    public private(set) var isEstablished = false
    public private(set) var isFIPS = false
    public private(set) var keyType: KeyEncType = .des

    public enum KeyEncType {
        case des   // Non-FIPS: 3DES
        case aes   // FIPS: AES-128
    }

    /// Block size for header padding, data encryption, and MAC (OpenSC-тай ижил)
    /// AES=16, DES=8 — бүх хэсэгт нэг block size ашиглана
    private var blockSize: Int { keyType == .aes ? 16 : 8 }

    public init() {}

    // MARK: - Mutual Authentication

    /// Бүрэн mutual auth: GET DATA → detect FIPS → INITIALIZE UPDATE → EXTERNAL AUTH
    public func mutualAuth(
        card: CardTransmitter,
        transportKeyEnc: [UInt8] = BioPassDriver.defaultTransportKey,
        transportKeyMac: [UInt8] = BioPassDriver.defaultTransportKey
    ) async throws {
        // Step 1: GET DATA (tag 0x0186) to detect FIPS + key type
        // OpenSC: 00 CA 01 86 00
        let getDataResp = try await card.transmitRaw(
            [0x00, 0xCA, 0x01, 0x86, 0x00]
        )
        if getDataResp.count > 4 {
            let data = Array(getDataResp.dropLast(2))
            // FIPS detection: data[0..3]="80 01 01" AND data[32..35]="87 01 01"
            if data.count > 34 && data[32] == 0x87 && data[33] == 0x01 && data[34] == 0x01
                && data[0] == 0x80 && data[1] == 0x01 && data[2] == 0x01 {
                isFIPS = true
            }
            // Key type: data[2]==0x01 → AES, else → DES (OpenSC epass2003_init)
            if data.count > 2 {
                keyType = data[2] == 0x01 ? .aes : .des
            }
        }

        // Step 2: Generate 8 bytes host random
        var hostRandom = [UInt8](repeating: 0, count: 8)
        guard SecRandomCopyBytes(kSecRandomDefault, 8, &hostRandom) == errSecSuccess else {
            throw TokenError.authFailed
        }

        // Step 3: INITIALIZE UPDATE (CLA=80 INS=50 P1=00 P2=00)
        var initAPDU = [UInt8](repeating: 0, count: 5 + 8)
        initAPDU[0] = 0x80; initAPDU[1] = 0x50; initAPDU[2] = 0x00; initAPDU[3] = 0x00
        initAPDU[4] = 0x08
        initAPDU.replaceSubrange(5..<13, with: hostRandom)

        let expectedLe = isFIPS ? 29 : 28
        initAPDU.append(UInt8(expectedLe))

        let initResp = try await card.transmitRaw(initAPDU)
        let initSW = Array(initResp.suffix(2))
        guard initSW[0] == 0x90 && initSW[1] == 0x00 else {
            throw TokenError.unexpectedStatus(UInt16(initSW[0]) << 8 | UInt16(initSW[1]))
        }

        let result = Array(initResp.dropLast(2))

        // Step 4: Extract card random (FIPS: offset+1)
        let cardRandom: [UInt8]
        if isFIPS {
            cardRandom = Array(result[13..<21])
        } else {
            cardRandom = Array(result[12..<20])
        }

        // Step 5: Derive session keys
        var derivationData = [UInt8](repeating: 0, count: 16)
        if isFIPS {
            // FIPS: offsets shifted by 1
            derivationData.replaceSubrange(0..<4, with: result[17..<21])
            derivationData.replaceSubrange(4..<8, with: hostRandom[0..<4])
            derivationData.replaceSubrange(8..<12, with: result[13..<17])
            derivationData.replaceSubrange(12..<16, with: hostRandom[4..<8])

            sessionKeyEnc = try aes128EncryptECB(key: transportKeyEnc, data: derivationData)
            sessionKeyMac = try aes128EncryptECB(key: transportKeyMac, data: derivationData)
        } else {
            derivationData.replaceSubrange(0..<4, with: result[16..<20])
            derivationData.replaceSubrange(4..<8, with: hostRandom[0..<4])
            derivationData.replaceSubrange(8..<12, with: result[12..<16])
            derivationData.replaceSubrange(12..<16, with: hostRandom[4..<8])

            if keyType == .aes {
                // Non-FIPS but AES key type (data[2]==0x01)
                sessionKeyEnc = try aes128EncryptECB(key: transportKeyEnc, data: derivationData)
                sessionKeyMac = try aes128EncryptECB(key: transportKeyMac, data: derivationData)
            } else {
                sessionKeyEnc = try des3EncryptECB(key: transportKeyEnc, data: derivationData)
                sessionKeyMac = try des3EncryptECB(key: transportKeyMac, data: derivationData)
            }
        }

        // Step 6: Calculate host cryptogram
        if isFIPS {
            // AES: 32-byte input (2 blocks of 16)
            var cryptInput = [UInt8](repeating: 0, count: 32)
            cryptInput.replaceSubrange(0..<8, with: hostRandom)
            cryptInput.replaceSubrange(8..<16, with: cardRandom)
            cryptInput[16] = 0x80

            let iv = [UInt8](repeating: 0, count: 16)
            let cryptogram = try aes128EncryptCBC(key: sessionKeyEnc, iv: iv, data: cryptInput)

            // Step 7: Verify card cryptogram
            let cardCryptogram = Array(result[21..<29])
            let expectedCardCrypt = Array(cryptogram[16..<24])
            guard constantTimeEquals(cardCryptogram, expectedCardCrypt) else {
                throw TokenError.authFailed
            }

            let hostCrypt = Array(cryptogram[16..<24])

            // Step 8: EXTERNAL AUTHENTICATE MAC (AES-CBC-MAC)
            var macInput = [UInt8](repeating: 0, count: 32)
            macInput[0] = 0x84; macInput[1] = 0x82; macInput[2] = 0x03; macInput[3] = 0x00; macInput[4] = 0x10
            macInput.replaceSubrange(5..<13, with: hostCrypt)
            macInput[13] = 0x80

            let macIV = [UInt8](repeating: 0, count: 16)
            let mac = try aesCBCMAC(key: sessionKeyMac, iv: macIV, data: macInput)

            icvMac = Array(repeating: 0, count: 16)
            // ICV-ийн СУУРЬ нь verify_init_key-ийн MAC (OpenSC: memcpy(icv_mac, &mac[i], 8)).
            icvMac.replaceSubrange(0..<8, with: mac)

            // Step 9: Send EXTERNAL AUTHENTICATE
            var extAuthAPDU = [UInt8](repeating: 0, count: 5 + 16)
            extAuthAPDU[0] = 0x84; extAuthAPDU[1] = 0x82; extAuthAPDU[2] = 0x03; extAuthAPDU[3] = 0x00
            extAuthAPDU[4] = 0x10
            extAuthAPDU.replaceSubrange(5..<13, with: hostCrypt)
            extAuthAPDU.replaceSubrange(13..<21, with: mac)

            let extAuthResp = try await card.transmitRaw(extAuthAPDU)
            let extAuthSW = Array(extAuthResp.suffix(2))
            guard extAuthSW[0] == 0x90 && extAuthSW[1] == 0x00 else {
                throw TokenError.authFailed
            }
        } else if keyType == .aes {
            // Non-FIPS AES: AES-128-CBC cryptogram, AES-CBC MAC
            var cryptInput = [UInt8](repeating: 0, count: 32) // 2 AES blocks
            cryptInput.replaceSubrange(0..<8, with: hostRandom)
            cryptInput.replaceSubrange(8..<16, with: cardRandom)
            cryptInput[16] = 0x80

            let iv16 = [UInt8](repeating: 0, count: 16)
            let cryptogram = try aes128EncryptCBC(key: sessionKeyEnc, iv: iv16, data: cryptInput)

            let cardCryptogram = Array(result[20..<28])
            let expectedCardCrypt = Array(cryptogram[16..<24])
            guard constantTimeEquals(cardCryptogram, expectedCardCrypt) else {
                throw TokenError.authFailed
            }

            // Host cryptogram: card_random + host_random (reversed order)
            var hostInput = [UInt8](repeating: 0, count: 32)
            hostInput.replaceSubrange(0..<8, with: cardRandom)
            hostInput.replaceSubrange(8..<16, with: hostRandom)
            hostInput[16] = 0x80
            let hostCryptogram = try aes128EncryptCBC(key: sessionKeyEnc, iv: iv16, data: hostInput)
            let hostCrypt = Array(hostCryptogram[16..<24])

            // MAC (AES-CBC-MAC)
            var macInput = [UInt8](repeating: 0, count: 16)
            macInput[0] = 0x84; macInput[1] = 0x82; macInput[2] = 0x03; macInput[3] = 0x00; macInput[4] = 0x10
            macInput.replaceSubrange(5..<13, with: hostCrypt)
            macInput[13] = 0x80

            let mac = try aesCBCMAC(key: sessionKeyMac, iv: iv16, data: macInput)

            icvMac = Array(repeating: 0, count: 16)
            icvMac.replaceSubrange(0..<8, with: mac)

            var extAuthAPDU = [UInt8](repeating: 0, count: 5 + 16)
            extAuthAPDU[0] = 0x84; extAuthAPDU[1] = 0x82; extAuthAPDU[2] = 0x03; extAuthAPDU[3] = 0x00
            extAuthAPDU[4] = 0x10
            extAuthAPDU.replaceSubrange(5..<13, with: hostCrypt)
            extAuthAPDU.replaceSubrange(13..<21, with: mac)

            let extAuthResp = try await card.transmitRaw(extAuthAPDU)
            let extAuthSW = Array(extAuthResp.suffix(2))
            guard extAuthSW[0] == 0x90 && extAuthSW[1] == 0x00 else {
                throw TokenError.authFailed
            }
        } else {
            // Non-FIPS DES: 3DES-CBC
            var cryptInput = [UInt8](repeating: 0, count: 24)
            cryptInput.replaceSubrange(0..<8, with: hostRandom)
            cryptInput.replaceSubrange(8..<16, with: cardRandom)
            cryptInput[16] = 0x80

            let iv = [UInt8](repeating: 0, count: 8)
            let cryptogram = try des3EncryptCBC(key: sessionKeyEnc, iv: iv, data: cryptInput)

            let cardCryptogram = Array(result[20..<28])
            let expectedCardCrypt = Array(cryptogram[16..<24])
            guard constantTimeEquals(cardCryptogram, expectedCardCrypt) else {
                throw TokenError.authFailed
            }

            // Host cryptogram: card_random + host_random (reversed)
            var hostInput = [UInt8](repeating: 0, count: 24)
            hostInput.replaceSubrange(0..<8, with: cardRandom)
            hostInput.replaceSubrange(8..<16, with: hostRandom)
            hostInput[16] = 0x80
            let hostCryptogram = try des3EncryptCBC(key: sessionKeyEnc, iv: iv, data: hostInput)
            let hostCrypt = Array(hostCryptogram[16..<24])

            var macInput = [UInt8](repeating: 0, count: 16)
            macInput[0] = 0x84; macInput[1] = 0x82; macInput[2] = 0x03; macInput[3] = 0x00; macInput[4] = 0x10
            macInput.replaceSubrange(5..<13, with: hostCrypt)
            macInput[13] = 0x80

            let macIV = [UInt8](repeating: 0, count: 8)
            let mac = try retailMAC(key: sessionKeyMac, iv: macIV, data: macInput)

            icvMac = Array(repeating: 0, count: 16)
            icvMac.replaceSubrange(0..<8, with: mac)

            var extAuthAPDU = [UInt8](repeating: 0, count: 5 + 16)
            extAuthAPDU[0] = 0x84; extAuthAPDU[1] = 0x82; extAuthAPDU[2] = 0x03; extAuthAPDU[3] = 0x00
            extAuthAPDU[4] = 0x10
            extAuthAPDU.replaceSubrange(5..<13, with: hostCrypt)
            extAuthAPDU.replaceSubrange(13..<21, with: mac)

            let extAuthResp = try await card.transmitRaw(extAuthAPDU)
            let extAuthSW = Array(extAuthResp.suffix(2))
            guard extAuthSW[0] == 0x90 && extAuthSW[1] == 0x00 else {
                throw TokenError.authFailed
            }
        }

        isEstablished = true
    }

    // MARK: - SM APDU Wrapping

    /// APDU-г SM-ээр wrap хийнэ (encrypt data + MAC).
    public func wrapAPDU(cla: UInt8, ins: UInt8, p1: UInt8, p2: UInt8,
                         data: [UInt8]? = nil, le: UInt8? = nil) throws -> [UInt8] {
        guard isEstablished else { throw TokenError.secureMessagingFailed }

        let bs = blockSize  // AES=16, DES=8

        // MAC header: CLA INS P1 P2 + 0x80 + padding to bs
        var macData = [UInt8](repeating: 0, count: bs)
        macData[0] = cla | 0x0C
        macData[1] = ins
        macData[2] = p1
        macData[3] = p2
        macData[4] = 0x80

        var dataTLV = [UInt8]()
        var leTLV = [UInt8]()

        // Encrypt data if present (pad to bs)
        if let data = data, !data.isEmpty {
            var padded = data
            padded.append(0x80)
            while padded.count % bs != 0 { padded.append(0x00) }

            let iv = [UInt8](repeating: 0, count: keyType == .aes ? 16 : 8)
            let encrypted: [UInt8]
            if keyType == .aes {
                encrypted = try aes128EncryptCBC(key: sessionKeyEnc, iv: iv, data: padded)
            } else {
                encrypted = try des3EncryptCBC(key: sessionKeyEnc, iv: iv, data: padded)
            }

            dataTLV.append(0x87)
            dataTLV.append(UInt8(encrypted.count + 1))
            dataTLV.append(0x01)
            dataTLV.append(contentsOf: encrypted)
        }

        // Le TLV if present
        if let le = le {
            leTLV = [0x97, 0x01, le]
        }

        // Calculate MAC (pad full input to bs)
        // OpenSC construct_mac_tlv: Data ба Le TLV ХОЁУЛАА байхгүй (case-1 APDU, ж: VERIFY
        // PIN-ий оролдлого тоолох `00 20 00 xx`) бол MAC нь ЗӨВХӨН 16 байт толгой дээр
        // тооцогдоно — 0x80 дүүргэлт НЭМЭГДЭХГҮЙ (`mac_len = block_size`). Дүүргэлт нэмбэл
        // карт SW=6988 буцаана.
        var fullMacInput = macData
        if !dataTLV.isEmpty || !leTLV.isEmpty {
            fullMacInput.append(contentsOf: dataTLV)
            fullMacInput.append(contentsOf: leTLV)
            fullMacInput.append(0x80)
            while fullMacInput.count % bs != 0 { fullMacInput.append(0x00) }
        }

        incrementICV()

        let mac: [UInt8]
        if keyType == .aes {
            mac = try aesCBCMAC(key: sessionKeyMac, iv: Array(icvMac.prefix(16)), data: fullMacInput)
        } else {
            mac = try retailMAC(key: sessionKeyMac, iv: Array(icvMac.prefix(8)), data: fullMacInput)
        }
        // (!) ICV-г MAC-аар ДАРЖ БИЧИХГҮЙ. OpenSC-ийн construct_mac_tlv нь ICV-г APDU
        // тутамд ЗӨВХӨН НЭГЭЭР НЭМДЭГ; суурь нь mutual auth-аас гарсан утга хэвээр
        // үлдэнэ. Урьд нь энд дарж бичдэг байсан тул 2 дахь APDU-гаас эхлэн ICV
        // картынхаас салж SW=6988 ("SM data objects incorrect") өгдөг байв.
        let macTLV: [UInt8] = [0x8E, 0x08] + mac

        let totalData = dataTLV + leTLV + macTLV
        var apdu: [UInt8] = [cla | 0x0C, ins, p1, p2, UInt8(totalData.count)]
        apdu.append(contentsOf: totalData)
        // OpenSC: Le байтыг ЗӨВХӨН Le TLV байгуулсан үед нэмнэ, УТГА нь Le TLV-тэйгээ ИЖИЛ
        // (`*(apdu_buf + …) = plain->le`). Урьд нь үргэлж 0x00 явуулдаг байсан тул
        // READ BINARY (le=0xD8) дээр карт SW=6A80 өгдөг байв.
        if let le = le, !leTLV.isEmpty { apdu.append(le) }

        return apdu
    }

    /// SM response decrypt хийнэ.
    public func unwrapResponse(_ response: [UInt8]) throws -> (data: [UInt8], sw1: UInt8, sw2: UInt8) {
        guard response.count >= 2 else { throw TokenError.secureMessagingFailed }

        let sw1 = response[response.count - 2]
        let sw2 = response[response.count - 1]

        guard sw1 == 0x90 && sw2 == 0x00 else {
            return ([], sw1, sw2)
        }

        let body = Array(response.dropLast(2))
        if body.isEmpty { return ([], sw1, sw2) }

        // BER-TLV: урт нь богино (0x00-0x7F) эсвэл урт хэлбэр (0x81/0x82 + байтууд) байна.
        // READ BINARY-ийн 254 байтын хариу нь урт хэлбэрээр ирдэг тул богино хэлбэр л
        // мэддэг задлагч 0x81/0x82-ыг УРТ гэж уншиж бүх хариуг алддаг байв.
        func berLength(_ body: [UInt8], _ i: Int) -> (len: Int, headerLen: Int)? {
            guard i + 1 < body.count else { return nil }
            let first = Int(body[i + 1])
            if first < 0x80 { return (first, 2) }
            let n = first & 0x7F
            guard n >= 1, n <= 3, i + 1 + n < body.count else { return nil }
            var v = 0
            for k in 1...n { v = (v << 8) | Int(body[i + 1 + k]) }
            return (v, 2 + n)
        }

        var decryptedData = [UInt8]()
        var i = 0
        while i < body.count {
            let tag = body[i]
            guard let (len, hdr) = berLength(body, i) else { break }
            let valueStart = i + hdr
            let valueEnd = valueStart + len
            guard valueEnd <= body.count else { break }

            if tag == 0x87 {
                // 87 | L | 01 | <шифрлэсэн> — L нь 0x01 индикаторыг ОРУУЛСАН урт.
                guard len >= 1 else { break }
                let encrypted = Array(body[(valueStart + 1)..<valueEnd])
                let block = keyType == .aes ? 16 : 8
                guard !encrypted.isEmpty, encrypted.count % block == 0 else { break }
                let iv = [UInt8](repeating: 0, count: block)
                let decrypted: [UInt8]
                if keyType == .aes {
                    decrypted = try aes128DecryptCBC(key: sessionKeyEnc, iv: iv, data: encrypted)
                } else {
                    decrypted = try des3DecryptCBC(key: sessionKeyEnc, iv: iv, data: encrypted)
                }
                decryptedData.append(contentsOf: removePadding(decrypted))
            }
            i = valueEnd
        }

        return (decryptedData, sw1, sw2)
    }

    /// BioPassDriver-ийн external key auth-д (PIN → 3DES) хэрэгтэй нийтийн боодол.
    static func des3EncryptCBCPublic(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        try Self.ccCrypt(op: CCOperation(kCCEncrypt), alg: CCAlgorithm(kCCAlgorithm3DES),
                    options: 0, key: key, iv: iv, data: data)
    }

    /// Оношилгоо: ICV-ийн одоогийн утга (SM тоолуурыг картынхтай тулгахад).
    public var debugICV: [UInt8] { icvMac }

    // MARK: - ICV management

    private func incrementICV() {
        // OpenSC: AES → byte 15-аас, DES → byte 7-аас increment
        let start = keyType == .aes ? 15 : 7
        for i in stride(from: start, through: 0, by: -1) {
            if icvMac[i] == 0xFF {
                icvMac[i] = 0
            } else {
                icvMac[i] += 1
                break
            }
        }
    }

    // MARK: - CommonCrypto wrapper

    static func ccCrypt(op: CCOperation, alg: CCAlgorithm, options: CCOptions,
                         key: [UInt8], iv: [UInt8]?, data: [UInt8]) throws -> [UInt8] {
        let blockSize = alg == CCAlgorithm(kCCAlgorithmAES128) ? kCCBlockSizeAES128 : kCCBlockSize3DES
        let outSize = data.count + blockSize
        var out = [UInt8](repeating: 0, count: outSize)
        var outLen = 0

        let status = CCCrypt(op, alg, options,
                             key, key.count,
                             iv, data, data.count,
                             &out, outSize, &outLen)
        guard status == kCCSuccess else { throw TokenError.secureMessagingFailed }
        return Array(out.prefix(outLen))
    }

    // MARK: - 3DES (Non-FIPS)

    private func des3EncryptECB(key: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16 else { throw TokenError.secureMessagingFailed }
        let key24 = key + Array(key.prefix(8))
        return try Self.ccCrypt(op: CCOperation(kCCEncrypt), alg: CCAlgorithm(kCCAlgorithm3DES),
                           options: CCOptions(kCCOptionECBMode), key: key24, iv: nil, data: data)
    }

    func des3EncryptCBC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16, iv.count == 8 else { throw TokenError.secureMessagingFailed }
        let key24 = key + Array(key.prefix(8))
        return try Self.ccCrypt(op: CCOperation(kCCEncrypt), alg: CCAlgorithm(kCCAlgorithm3DES),
                           options: 0, key: key24, iv: iv, data: data)
    }

    private func des3DecryptCBC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16, iv.count == 8 else { throw TokenError.secureMessagingFailed }
        let key24 = key + Array(key.prefix(8))
        return try Self.ccCrypt(op: CCOperation(kCCDecrypt), alg: CCAlgorithm(kCCAlgorithm3DES),
                           options: 0, key: key24, iv: iv, data: data)
    }

    private func desEncryptCBC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        return try Self.ccCrypt(op: CCOperation(kCCEncrypt), alg: CCAlgorithm(kCCAlgorithmDES),
                           options: 0, key: key, iv: iv, data: data)
    }

    private func desDecryptCBC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        return try Self.ccCrypt(op: CCOperation(kCCDecrypt), alg: CCAlgorithm(kCCAlgorithmDES),
                           options: 0, key: key, iv: iv, data: data)
    }

    /// Retail MAC (ISO 9797-1 Algorithm 3): DES-CBC with final 3DES
    private func retailMAC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16 else { throw TokenError.secureMessagingFailed }
        let k1 = Array(key.prefix(8))
        let k2 = Array(key.suffix(8))

        let intermediate = try desEncryptCBC(key: k1, iv: iv, data: data)
        let lastBlock = Array(intermediate.suffix(8))

        let zeroIV = [UInt8](repeating: 0, count: 8)
        let decrypted = try desDecryptCBC(key: k2, iv: zeroIV, data: lastBlock)
        let mac = try desEncryptCBC(key: k1, iv: zeroIV, data: Array(decrypted.prefix(8)))
        return Array(mac.prefix(8))
    }

    // MARK: - AES-128 (FIPS)

    private func aes128EncryptECB(key: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16 else { throw TokenError.secureMessagingFailed }
        return try Self.ccCrypt(op: CCOperation(kCCEncrypt), alg: CCAlgorithm(kCCAlgorithmAES128),
                           options: CCOptions(kCCOptionECBMode), key: key, iv: nil, data: data)
    }

    private func aes128EncryptCBC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16, iv.count == 16 else { throw TokenError.secureMessagingFailed }
        return try Self.ccCrypt(op: CCOperation(kCCEncrypt), alg: CCAlgorithm(kCCAlgorithmAES128),
                           options: 0, key: key, iv: iv, data: data)
    }

    private func aes128DecryptCBC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16, iv.count == 16 else { throw TokenError.secureMessagingFailed }
        return try Self.ccCrypt(op: CCOperation(kCCDecrypt), alg: CCAlgorithm(kCCAlgorithmAES128),
                           options: 0, key: key, iv: iv, data: data)
    }

    /// AES-CBC-MAC: last block of AES-CBC, truncated to 8 bytes.
    private func aesCBCMAC(key: [UInt8], iv: [UInt8], data: [UInt8]) throws -> [UInt8] {
        guard key.count == 16 else { throw TokenError.secureMessagingFailed }
        let encrypted = try aes128EncryptCBC(key: key, iv: iv, data: data)
        return Array(encrypted.suffix(16).prefix(8))
    }

    // MARK: - Helpers

    private func removePadding(_ data: [UInt8]) -> [UInt8] {
        guard let idx = data.lastIndex(of: 0x80) else { return data }
        let afterPad = data[(idx + 1)...]
        if afterPad.allSatisfy({ $0 == 0x00 }) {
            return Array(data.prefix(idx))
        }
        return data
    }
}

// MARK: - Card Transmitter Protocol

/// PCSC level raw APDU transmit protocol.
public protocol CardTransmitter {
    func transmitRaw(_ apdu: [UInt8]) async throws -> [UInt8]
}
