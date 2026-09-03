import Foundation
import CryptoTokenKit
import CommonCrypto
import CryptoKit
import os

/// BioPass2003 / ePass2003 токений APDU-level driver.
/// OpenSC card-epass2003.c-ийн Swift хувилбар.
///
/// Applet: EnterSafe-FIPS
/// AID: 65:6E:74:65:72:73:61:66:65:2D:66:69:70:73
///
/// Mutual auth дараа бүх APDU Secure Messaging (3DES) ашигладаг.
public final class BioPassDriver {

    /// EnterSafe-FIPS applet AID
    public static let appletAID: [UInt8] = [
        0x65, 0x6E, 0x74, 0x65, 0x72, 0x73, 0x61, 0x66,
        0x65, 0x2D, 0x66, 0x69, 0x70, 0x73
    ]

    /// Factory default transport key (encryption + MAC)
    /// OpenSC card-epass2003.c line 94-102
    public static let defaultTransportKey: [UInt8] = [
        0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
        0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10
    ]

    private let logger = Logger(subsystem: "mn.gerege.token-kit", category: "BioPassDriver")

    /// Идэвхтэй SM session (mutual auth дараа тогтоогдоно)
    public private(set) var sm: SecureMessaging?

    public init() {}

    // MARK: - SELECT Applet

    /// EnterSafe-FIPS applet сонгох.
    /// APDU: 00 A4 04 00 0E [AID]
    public func selectApplet(card: TKSmartCard) async throws -> APDUResponse {
        let cmd = try APDUCommand(
            cla: 0x00, ins: 0xA4, p1: 0x04, p2: 0x00,
            data: Self.appletAID
        )
        let response = try await APDUTransceiver.transmit(cmd, to: card)
        logger.info("SELECT applet: SW=\(response.hexStatus)")
        return response
    }

    // MARK: - GET CHALLENGE

    /// 8 байт random challenge авах.
    /// APDU: 00 84 00 00 08
    public func getChallenge(card: TKSmartCard) async throws -> [UInt8] {
        // SM тогтоогдсон бол ЗААВАЛ wrap хийнэ — эс бөгөөс карт татгалзана (SM session
        // идэвхтэй үед ил APDU хүлээж авдаггүй). transmitWithSM нь SM байхгүй үед ил явуулна.
        let response = try await transmitWithSM(cla: 0x00, ins: 0x84, p1: 0x00, p2: 0x00,
                                                data: nil, le: 0x08, card: card)

        guard response.isSuccess, let data = response.data, data.count == 8 else {
            logger.error("GET CHALLENGE failed: SW=\(response.hexStatus)")
            throw TokenError.authFailed
        }

        return data
    }

    // MARK: - Mutual Authentication + Secure Messaging

    /// SM session тогтоох — mutual auth хийж session key-үүд авна.
    /// Дараагийн бүх APDU автоматаар SM wrap хийгдэнэ.
    public func establishSecureSession(card: TKSmartCard,
                                        transportKeyEnc: [UInt8] = BioPassDriver.defaultTransportKey,
                                        transportKeyMac: [UInt8] = BioPassDriver.defaultTransportKey) async throws {
        let transmitter = TKSmartCardTransmitter(card: card)
        let session = SecureMessaging()
        try await session.mutualAuth(card: transmitter,
                                      transportKeyEnc: transportKeyEnc,
                                      transportKeyMac: transportKeyMac)
        self.sm = session
        logger.info("SM session тогтоогдлоо (FIPS=\(session.isFIPS))")
    }

    /// SM session байгаа бол APDU-г wrap хийж илгээнэ, байхгүй бол шууд илгээнэ.
    private func transmitWithSM(cla: UInt8, ins: UInt8, p1: UInt8, p2: UInt8,
                                 data: [UInt8]? = nil, le: UInt8? = nil,
                                 card: TKSmartCard) async throws -> APDUResponse {
        if let sm = self.sm, sm.isEstablished {
            let transmitter = TKSmartCardTransmitter(card: card)
            let wrapped = try sm.wrapAPDU(cla: cla, ins: ins, p1: p1, p2: p2, data: data, le: le)
            let rawResp = try await transmitter.transmitRaw(wrapped)
            let (decrypted, sw1, sw2) = try sm.unwrapResponse(rawResp)
            return APDUResponse(data: decrypted.isEmpty ? nil : decrypted, sw1: sw1, sw2: sw2)
        } else {
            let cmd = try APDUCommand(cla: cla, ins: ins, p1: p1, p2: p2, data: data, le: le ?? 0)
            return try await APDUTransceiver.transmit(cmd, to: card)
        }
    }

    // MARK: - VERIFY PIN

    /// User PIN шалгах.
    /// APDU: 00 20 00 [ref] [Lc] [PIN bytes padded to 16]
    /// ref: 0x01 = SO PIN, 0x02 = User PIN
    public func verifyPIN(_ pin: String, reference: UInt8 = 0x02, card: TKSmartCard) async throws {
        var pinBytes = Array(pin.utf8)
        // Pad to 16 bytes with 0x00
        while pinBytes.count < 16 { pinBytes.append(0x00) }
        if pinBytes.count > 16 { pinBytes = Array(pinBytes.prefix(16)) }

        let response = try await transmitWithSM(
            cla: 0x00, ins: 0x20, p1: 0x00, p2: reference,
            data: pinBytes, card: card
        )

        // Wipe PIN bytes from memory
        for i in 0..<pinBytes.count { pinBytes[i] = 0 }

        if response.isSuccess {
            logger.info("VERIFY PIN: амжилттай")
            return
        }

        // 63 Cx = wrong PIN, x = retries left
        if response.sw1 == 0x63 && (response.sw2 & 0xF0) == 0xC0 {
            let retries = Int(response.sw2 & 0x0F)
            throw TokenError.pinVerifyFailed(retriesLeft: retries)
        }

        // 69 83 = PIN blocked
        if response.sw1 == 0x69 && response.sw2 == 0x83 {
            throw TokenError.pinBlocked
        }

        throw TokenError.unexpectedStatus(response.status)
    }

    // MARK: - Token Info

    /// Токений мэдээлэл авах — ATR, label, serial.
    public func getTokenInfo(card: TKSmartCard) async throws -> TokenInfo {
        let atr: [UInt8]
        if let atrData = card.slot.atr {
            atr = Array(atrData.bytes)
        } else {
            atr = []
        }

        // SELECT applet
        let selectResp = try await selectApplet(card: card)

        // Try to read token label from EF 2440
        var label = "BioPass2003"
        do {
            let readCmd = try APDUCommand(hex: "00B000002800")
            let readResp = try await APDUTransceiver.transmit(readCmd, to: card)
            if readResp.isSuccess, let data = readResp.data {
                if let str = String(bytes: data.prefix(while: { $0 != 0x00 && $0 != 0xFF }), encoding: .utf8),
                   !str.isEmpty {
                    label = str.trimmingCharacters(in: .whitespaces)
                }
            }
        } catch {
            // Label уншиж чадахгүй бол default ашиглана
        }

        let isFIPS = atr.count > 19 && atr[19] == 0x03

        return TokenInfo(
            atr: atr,
            label: label,
            isFIPS: isFIPS,
            appletSelected: selectResp.isSuccess
        )
    }

    // MARK: - SIGN

    /// ECDSA гарын үсэг зурах.
    /// APDU: 00 2A 9E 9A [hash]
    /// SM session байгаа бол SM wrap хийнэ.
    public func signECDSA(hash: [UInt8], card: TKSmartCard) async throws -> [UInt8] {
        let response = try await transmitWithSM(
            cla: 0x00, ins: 0x2A, p1: 0x9E, p2: 0x9A,
            data: hash, card: card
        )

        guard response.isSuccess, let signature = response.data else {
            logger.error("SIGN failed: SW=\(response.hexStatus)")
            throw TokenError.signFailed("SW=\(response.hexStatus)")
        }

        return signature
    }

    /// RSA гарын үсэг зурах.
    /// APDU: 00 2A 80 86 [data]
    public func signRSA(data: [UInt8], card: TKSmartCard) async throws -> [UInt8] {
        let response = try await transmitWithSM(
            cla: 0x00, ins: 0x2A, p1: 0x80, p2: 0x86,
            data: data, card: card
        )

        guard response.isSuccess, let signature = response.data else {
            logger.error("SIGN RSA failed: SW=\(response.hexStatus)")
            throw TokenError.signFailed("SW=\(response.hexStatus)")
        }

        return signature
    }

    // MARK: - Token Initialization (Phase 2)
    // OpenSC epass2003 internal_install_pre + internal_install_pin порт.

    // Access control constants (OpenSC cardctl.h)
    private static let AC_EVERYONE: UInt8 = 0x00
    private static let AC_USER: UInt8 = 0x06
    private static let AC_SO: UInt8 = 0x08
    private static let AC_MAC_NOLESS: UInt8 = 0x90
    private static let USER_PIN_ID: UInt8 = 0x01
    private static let SO_PIN_ID: UInt8 = 0x02

    /// MF AID (EnterSafe-FIPS, OpenSC epass2003.profile-тай ижил)
    public static let mfAID: [UInt8] = [
        0x65, 0x6E, 0x74, 0x65, 0x72, 0x73, 0x61, 0x66,
        0x65, 0x2D, 0x66, 0x69, 0x70, 0x73
    ]

    /// File IDs (OpenSC epass2003.profile)
    public static let FID_MF: UInt16       = 0x3F00
    public static let FID_EF_DIR: UInt16   = 0x2F00
    public static let FID_SKEY_MF: UInt16  = 0x5300
    public static let FID_APP_DF: UInt16   = 0x5015   // PKCS#15 DF (default AID last 2 bytes)
    public static let FID_SKEY_APP: UInt16 = 0x5301
    public static let FID_MAXPIN: UInt16   = 0x9F00
    public static let FID_ODF: UInt16      = 0x5031
    public static let FID_TOKEN_INFO: UInt16 = 0x5032
    public static let FID_UNUSED: UInt16   = 0x5033
    public static let FID_AODF: UInt16     = 0x4401
    public static let FID_PRKDF: UInt16    = 0x4402
    public static let FID_PUKDF: UInt16    = 0x4403
    public static let FID_CDF: UInt16      = 0x4404
    public static let FID_DODF: UInt16     = 0x4405

    // File type codes (ISO 7816-4 + EnterSafe extensions)
    public static let FILETYPE_DF: UInt8         = 0x38
    public static let FILETYPE_EF_TRANS: UInt8   = 0x01
    public static let FILETYPE_EF_LINFIX: UInt8  = 0x02
    public static let FILETYPE_EF_LINVAR: UInt8  = 0x04
    public static let FILETYPE_EF_INT_EC: UInt8  = 0x0A
    public static let FILETYPE_EF_INT_RSA: UInt8 = 0x11

    /// Токен бүрэн initialize — erase + transport keys + filesystem + PINs.
    /// Энэ нь OpenSC `pkcs15-init --create-pkcs15` + `--store-pin`-ын үйлдлийн дараалал.
    public func initializePIN(pin: String, puk: String, card: TKSmartCard) async throws {
        guard sm?.isEstablished == true else { throw TokenError.authFailed }

        // Step 1: Erase existing data (OpenSC epass2003_erase_card)
        try await eraseCard(card: card)
        logger.info("Card erased")

        // Step 2: Install transport keys (OpenSC internal_install_pre)
        try await installSecretKey(ktype: 0x01, kid: 0x00,
                                    useac: Self.AC_MAC_NOLESS | Self.AC_EVERYONE,
                                    modifyac: Self.AC_MAC_NOLESS | Self.AC_EVERYONE,
                                    ec: 0, data: Self.defaultTransportKey, card: card)
        logger.info("Transport ENC key installed")

        try await installSecretKey(ktype: 0x02, kid: 0x00,
                                    useac: Self.AC_MAC_NOLESS | Self.AC_EVERYONE,
                                    modifyac: Self.AC_MAC_NOLESS | Self.AC_EVERYONE,
                                    ec: 0, data: Self.defaultTransportKey, card: card)
        logger.info("Transport MAC key installed")

        // Step 3: Create PKCS#15 filesystem (OpenSC epass2003_pkcs15_init_card)
        try await createPKCS15Filesystem(card: card)
        logger.info("PKCS#15 filesystem created")

        // Step 4: Install SO PIN (OpenSC internal_install_pin)
        let soHash = hashPINForInstall(puk)
        try await installSecretKey(ktype: 0x04, kid: Self.SO_PIN_ID,
                                    useac: Self.AC_MAC_NOLESS | Self.AC_SO,
                                    modifyac: Self.AC_MAC_NOLESS | Self.AC_SO,
                                    ec: 10, data: soHash, card: card)
        logger.info("SO PIN (PUK) installed")

        // Step 5: Install User PIN
        let userHash = hashPINForInstall(pin)
        try await installSecretKey(ktype: 0x04, kid: Self.USER_PIN_ID,
                                    useac: Self.AC_MAC_NOLESS | Self.AC_USER,
                                    modifyac: Self.AC_MAC_NOLESS | Self.AC_SO,
                                    ec: 10, data: userHash, card: card)
        logger.info("User PIN installed")
    }

    // MARK: - ERASE CARD (80 0E 00 00)

    /// Токен бүх хэрэглэгчийн data-г устгана. SM заавал шаардлагатай.
    /// OpenSC: epass2003_erase_card
    public func eraseCard(card: TKSmartCard) async throws {
        let response = try await transmitWithSM(
            cla: 0x80, ins: 0x0E, p1: 0x00, p2: 0x00,
            data: [], card: card
        )
        guard response.isSuccess else {
            logger.error("ERASE CARD failed: SW=\(response.hexStatus)")
            throw TokenError.unexpectedStatus(response.status)
        }
    }

    // MARK: - CREATE FILE (80 E0)

    /// ISO 7816-4 CREATE FILE APDU. FCP TLV нь `62 [len] ...` format.
    /// OpenSC: epass2003_create_file.
    public func createFile(fcp: [UInt8], card: TKSmartCard) async throws {
        let response = try await transmitWithSM(
            cla: 0x80, ins: 0xE0, p1: 0x00, p2: 0x00,
            data: fcp, card: card
        )
        guard response.isSuccess else {
            logger.error("CREATE FILE failed: SW=\(response.hexStatus)")
            throw TokenError.unexpectedStatus(response.status)
        }
    }

    // MARK: - SELECT FILE by FID

    /// SELECT FILE by 2-byte File ID (00 A4 00 00).
    public func selectFile(fid: UInt16, card: TKSmartCard) async throws -> APDUResponse {
        let fidBytes: [UInt8] = [UInt8(fid >> 8), UInt8(fid & 0xFF)]
        let response = try await transmitWithSM(
            cla: 0x00, ins: 0xA4, p1: 0x00, p2: 0x00,
            data: fidBytes, card: card
        )
        guard response.isSuccess else {
            logger.error("SELECT FID \(String(format: "%04X", fid)) failed: SW=\(response.hexStatus)")
            throw TokenError.unexpectedStatus(response.status)
        }
        return response
    }

    // MARK: - FCP (File Control Parameters) builders

    /// DF (Dedicated File) FCP бүтээх.
    /// OpenSC `construct_fci` with DF template.
    public static func buildDFFCP(fid: UInt16, size: Int, aid: [UInt8]? = nil,
                                   acl: [UInt8] = [0xFF, 0xFF, 0x04, 0x44, 0x44, 0x44, 0x44, 0x44]) -> [UInt8] {
        var inner = [UInt8]()
        // 80 02 [size_hi] [size_lo]
        inner += [0x80, 0x02, UInt8(size >> 8), UInt8(size & 0xFF)]
        // 82 02 [file_type] 00
        inner += [0x82, 0x02, FILETYPE_DF, 0x00]
        // 83 02 [fid_hi] [fid_lo]
        inner += [0x83, 0x02, UInt8(fid >> 8), UInt8(fid & 0xFF)]
        // 84 [len] [AID]
        if let aid = aid, !aid.isEmpty {
            inner += [0x84, UInt8(aid.count)]
            inner += aid
        }
        // 8A 01 01 (life cycle = operational activated)
        inner += [0x8A, 0x01, 0x01]
        // 8C [len] [ACL]
        inner += [0x8C, UInt8(acl.count)] + acl
        // 62 [len] [inner]
        return [0x62, UInt8(inner.count)] + inner
    }

    /// EF (Elementary File) FCP бүтээх.
    public static func buildEFFCP(fid: UInt16, size: Int, fileType: UInt8 = FILETYPE_EF_TRANS,
                                   acl: [UInt8] = [0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]) -> [UInt8] {
        var inner = [UInt8]()
        inner += [0x80, 0x02, UInt8(size >> 8), UInt8(size & 0xFF)]
        inner += [0x82, 0x02, fileType, 0x00]
        inner += [0x83, 0x02, UInt8(fid >> 8), UInt8(fid & 0xFF)]
        inner += [0x8A, 0x01, 0x01]
        inner += [0x8C, UInt8(acl.count)] + acl
        return [0x62, UInt8(inner.count)] + inner
    }

    // MARK: - PKCS#15 Filesystem

    /// OpenSC epass2003.profile-тай ижил бүтэц бий болгоно:
    /// MF(3F00) → EF DIR(2F00), SKey-MF(5300), DF PKCS15-App(5015)
    /// DF PKCS15-App → SKey-App(5301), MAXPIN(9F00), ODF(5031), TokenInfo(5032),
    ///                 UnusedSpace(5033), AODF(4401), PrKDF(4402), PuKDF(4403),
    ///                 CDF(4404), DODF(4405)
    public func createPKCS15Filesystem(card: TKSmartCard) async throws {
        // SELECT MF first (токен дээр MF үргэлж бий)
        _ = try await selectFile(fid: Self.FID_MF, card: card)

        // EF DIR (linear-variable, 112 bytes)
        try await createFile(
            fcp: Self.buildEFFCP(fid: Self.FID_EF_DIR, size: 112,
                                  fileType: Self.FILETYPE_EF_LINVAR),
            card: card
        )
        logger.info("Created EF DIR (2F00)")

        // DF PKCS#15 application (size=16000, with AID)
        try await createFile(
            fcp: Self.buildDFFCP(fid: Self.FID_APP_DF, size: 16000, aid: Self.mfAID),
            card: card
        )
        logger.info("Created DF PKCS15-App (5015)")

        // SELECT into app DF
        _ = try await selectFile(fid: Self.FID_APP_DF, card: card)

        // Core PKCS#15 EFs
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_MAXPIN, size: 2), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_ODF, size: 256), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_TOKEN_INFO, size: 128), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_UNUSED, size: 128), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_AODF, size: 256), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_PRKDF, size: 256), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_PUKDF, size: 256), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_CDF, size: 512), card: card)
        try await createFile(fcp: Self.buildEFFCP(fid: Self.FID_DODF, size: 256), card: card)
        logger.info("Created 9 PKCS#15 EFs inside App DF")
    }

    /// OpenSC install_secret_key — APDU: 80 E3 00 00 [data]
    private func installSecretKey(ktype: UInt8, kid: UInt8, useac: UInt8, modifyac: UInt8,
                                   ec: UInt8, data: [UInt8], card: TKSmartCard) async throws {
        var tmp = [UInt8](repeating: 0, count: 10 + data.count)
        tmp[0] = ktype
        tmp[1] = kid
        tmp[2] = useac
        tmp[3] = modifyac
        tmp[8] = 0xFF

        if ktype == 0x04 || ktype == 0x06 {
            // PIN type
            tmp[4] = Self.AC_MAC_NOLESS | Self.AC_SO
            tmp[5] = Self.AC_MAC_NOLESS | Self.AC_SO
            tmp[7] = kid == Self.USER_PIN_ID ? Self.AC_USER : Self.AC_SO
            tmp[9] = (ec << 4) | ec
        }

        tmp.replaceSubrange(10..<(10 + data.count), with: data)

        let response = try await transmitWithSM(
            cla: 0x80, ins: 0xE3, p1: 0x00, p2: 0x00,
            data: tmp, card: card
        )

        guard response.isSuccess else {
            logger.error("install_secret_key(ktype=\(String(format: "%02X", ktype))) failed: SW=\(response.hexStatus)")
            throw TokenError.unexpectedStatus(response.status)
        }
    }

    /// PIN → SHA-1 hash (24 bytes: hash(20) + length(4))
    /// OpenSC hash_data with SC_ALGORITHM_ECDSA_HASH_SHA1
    private func hashPINForInstall(_ pin: String) -> [UInt8] {
        let pinData = Array(pin.utf8)
        var hash = [UInt8](repeating: 0, count: 20)
        let len = pinData.count
        CC_SHA1(pinData, CC_LONG(pinData.count), &hash)
        // Append reversed length (4 bytes, little-endian)
        let lenBytes: [UInt8] = [
            UInt8(len & 0xFF), UInt8((len >> 8) & 0xFF),
            UInt8((len >> 16) & 0xFF), UInt8((len >> 24) & 0xFF)
        ]
        return hash + lenBytes  // 24 bytes total (HASH_LEN)
    }

    // MARK: - Key Generation (Phase 3)

    /// EC key pair үүсгэх — SM session шаардлагатай.
    /// APDU: 00 46 00 00 [key spec TLV]
    public func generateECKeyPair(keyID: UInt8 = 0x01, card: TKSmartCard) async throws {
        guard sm?.isEstablished == true else {
            throw TokenError.authFailed
        }

        // Key generation spec TLV
        // OpenSC: INS=0x46, key type + size
        let keySpec: [UInt8] = [
            0x30, 0x08,                         // SEQUENCE
            0x04, 0x01, keyID,                  // key ID
            0x80, 0x01, 0x13,                   // algorithm: EC P-256 (0x13)
            0x81, 0x01, 0x20,                   // key size: 32 bytes (256 bit)
        ]

        let response = try await transmitWithSM(
            cla: 0x00, ins: 0x46, p1: 0x00, p2: 0x00,
            data: keySpec, card: card
        )

        guard response.isSuccess else {
            logger.error("GENERATE KEY PAIR failed: SW=\(response.hexStatus)")
            throw TokenError.keyGenFailed
        }

        logger.info("EC key pair үүсгэгдлэа (keyID=\(String(format: "%02X", keyID)))")
    }

    /// RSA key pair үүсгэх — SM session шаардлагатай.
    /// APDU: 00 46 00 00 [key spec TLV]
    public func generateRSAKeyPair(keyID: UInt8 = 0x02, keySize: Int = 2048, card: TKSmartCard) async throws {
        guard sm?.isEstablished == true else {
            throw TokenError.authFailed
        }

        let keySizeBytes = UInt16(keySize)
        let keySpec: [UInt8] = [
            0x30, 0x08,
            0x04, 0x01, keyID,
            0x80, 0x01, 0x01,                   // algorithm: RSA (0x01)
            0x81, 0x02, UInt8(keySizeBytes >> 8), UInt8(keySizeBytes & 0xFF),
        ]

        let response = try await transmitWithSM(
            cla: 0x00, ins: 0x46, p1: 0x00, p2: 0x00,
            data: keySpec, card: card
        )

        guard response.isSuccess else {
            logger.error("GENERATE RSA KEY PAIR failed: SW=\(response.hexStatus)")
            throw TokenError.keyGenFailed
        }

        logger.info("RSA key pair үүсгэгдлэа (keyID=\(String(format: "%02X", keyID)), size=\(keySize))")
    }

    // MARK: - Read Certificate

    /// Токеноос сертификат уншиx.
    /// APDU: 00 B0 [offset_hi] [offset_lo] [len]
    public func readCertificate(fileID: UInt16 = 0x2441, card: TKSmartCard) async throws -> [UInt8] {
        // SELECT file
        let selectData: [UInt8] = [UInt8(fileID >> 8), UInt8(fileID & 0xFF)]
        let selectResp = try await transmitWithSM(
            cla: 0x00, ins: 0xA4, p1: 0x00, p2: 0x00,
            data: selectData, card: card
        )
        guard selectResp.isSuccess else {
            throw TokenError.certificateNotFound
        }

        // Read binary in chunks
        var result = [UInt8]()
        var offset: UInt16 = 0
        let chunkSize: UInt8 = 0xFE

        while true {
            let response = try await transmitWithSM(
                cla: 0x00, ins: 0xB0,
                p1: UInt8(offset >> 8), p2: UInt8(offset & 0xFF),
                le: chunkSize, card: card
            )

            if let data = response.data, !data.isEmpty {
                result.append(contentsOf: data)
                offset += UInt16(data.count)
                if data.count < Int(chunkSize) { break }
            } else {
                break
            }
        }

        guard !result.isEmpty else {
            throw TokenError.certificateNotFound
        }

        return result
    }

    /// Токен дээрх БҮХ гэрчилгээг уншина (нэг токен дээр auth + sign гэсэн 2 гэрчилгээ
    /// байдаг — Windows клиент хоёуланг нь харуулдаг).
    ///
    /// ЯАГААД сканнердаж байна вэ: OpenSC энэ картыг PKCS#15 гэж бинд хийж чаддаггүй
    /// (`pkcs15-tool --dump` → "Unsupported card") тул CDF-ийг ердийн замаар уншиж
    /// чадахгүй. Гэрчилгээний EF-үүд EnterSafe-FIPS layout-д дараалсан ID-тай байдаг
    /// тул тодорхой БАГА мужийг уншиж, X.509 DER мэт эхэлж буйг нь л авна.
    ///
    /// Зөвхөн УНШИНА (00 A4 SELECT + 00 B0 READ BINARY) — бичихгүй.
    public func readAllCertificates(
        fileIDs: [UInt16] = [0x2441, 0x2442, 0x2443, 0x2444, 0x2445, 0x2446],
        card: TKSmartCard
    ) async throws -> [(fileID: UInt16, der: [UInt8])] {
        var out: [(UInt16, [UInt8])] = []
        for fid in fileIDs {
            guard let der = try? await readCertificate(fileID: fid, card: card) else { continue }
            // X.509 нь ЗААВАЛ SEQUENCE (0x30)-ээр эхэлнэ. Хоосон/дүүргэсэн EF-ийг хаяна.
            guard der.count > 64, der[0] == 0x30 else { continue }
            out.append((fid, der))
        }
        return out
    }

    /// EnterSafe-**ESPK** байршуулалттай токеноос гэрчилгээ уншина (Tridium/Gerege Signature
    /// зэрэг ҮГ-ийн олгосон карт). Гэрэгэ-гийн өөрийн FIPS токеноос ЯЛГААТАЙ:
    ///
    ///   MF 3F00 → DF 2003 ("ENTERSAFE-ESPK") → EF 24C0 / 24E0 …
    ///
    /// `selectApplet()` нь `entersafe-fips` AID сонгодог тул эдгээр карт дээр 6A82 буцааж,
    /// дараагийн mutual auth SW=9403-аар унадаг байв. Энд applet сонгохгүй, шууд FID-ээр
    /// орно — SM ч, PIN ч ШААРДЛАГАГҮЙ (гэрчилгээ нийтийн объект; бодит картан дээр
    /// SELECT/READ BINARY бүгд 9000 буцаана).
    ///
    /// EF-ийн эхэнд хувьсах урттай толгой байдаг (жишээ: 24C0 → 87 байт, 24E0 → 58) тул
    /// X.509 DER-ийн эхлэлийг (30 82 LL LL) хайж олоод уртаар нь тасална.
    public func readESPKCertificates(
        dfID: UInt16 = 0x2003,
        fileIDs: [UInt16] = [0x2480, 0x24A0, 0x24C0, 0x24E0],
        card: TKSmartCard
    ) async throws -> [(fileID: UInt16, der: [UInt8])] {
        // Карт нь SM тогтоогүй үед файл уншуулахгүй (энгийн SELECT → SW=6985). OpenSC ч
        // холбогдох үедээ ЯГ ижил mutual auth-ыг factory transport key-ээр хийдэг тул
        // эхлээд түүнийг оролдоно; амжилтгүй бол SM-гүйгээр үргэлжилнэ (зарим карт
        // гэрчилгээг ил өгдөг).
        if sm == nil {
            try? await establishSecureSession(card: card)
        }
        try await selectByFID(0x3F00, card: card)   // MF
        try await selectByFID(dfID, card: card)     // ENTERSAFE-ESPK

        var out: [(UInt16, [UInt8])] = []
        for fid in fileIDs {
            guard (try? await selectByFID(fid, card: card)) != nil else { continue }
            guard let raw = try? await readBinaryFully(card: card), !raw.isEmpty else { continue }
            guard let der = Self.extractDER(raw) else { continue }
            out.append((fid, der))
        }
        return out
    }

    /// Оношилгоо: ESPK замын алхам бүрийн SW. Картгүйгээр туршиж болдоггүй SM гинж
    /// (mutual auth → SELECT → READ) хаана тасарсныг харуулна.
    public func debugESPKTrace(card: TKSmartCard) async -> [String] {
        var t: [String] = []
        do {
            try await establishSecureSession(card: card)
            t.append("SM: OK (FIPS=\(sm?.isFIPS == true), \(sm?.keyType == .aes ? "AES" : "DES"))")
        } catch {
            t.append("SM: \(error.localizedDescription)")
            return t
        }
        for fid in [UInt16(0x3F00), 0x2003, 0x24C0] {
            do {
                try await selectByFID(fid, card: card)
                t.append(String(format: "SELECT %04X: 9000", fid))
            } catch {
                t.append(String(format: "SELECT %04X: ", fid) + error.localizedDescription)
                return t
            }
        }
        if let raw = try? await readBinaryFully(card: card) {
            t.append("READ 24C0: \(raw.count) байт → DER \(Self.extractDER(raw)?.count ?? 0)")
        }
        return t
    }

    /// Оношилгоо: PIN-ийн reference болон ҮЛДСЭН ОРОЛДЛОГЫГ аюулгүй тогтооно.
    /// `00 20 00 <ref>` (өгөгдөлгүй VERIFY) нь ISO 7816-д оролдлого ЗАРЦУУЛАХГҮЙГЭЭР
    /// SW=63Cx (x = үлдсэн) эсвэл 6A88 ("ийм PIN алга") буцаадаг.
    public func debugPINReferences(card: TKSmartCard, context: String = "") async -> [String] {
        var t: [String] = []
        for ref in [UInt8(0x00), 0x01, 0x02, 0x03, 0x80, 0x81, 0x82] {
            guard let r = try? await transmitWithSM(cla: 0x00, ins: 0x20, p1: 0x00, p2: ref,
                                                    data: nil, le: nil, card: card) else { continue }
            let sw = r.status
            let note: String
            switch sw >> 8 {
            case 0x63: note = "ҮЛДСЭН \(sw & 0x0F) оролдлого"
            case 0x69: note = "хаалттай/нөхцөл"
            case 0x6A: note = "байхгүй"
            default:   note = ""
            }
            t.append(context + String(format: " PIN ref %02X: SW=%04X %@", ref, sw, note))
        }
        return t
    }


    // MARK: - EnterSafe External Key Authentication (PIN)

    /// PIN-ийн ҮЛДСЭН ОРОЛДЛОГО — PIN явуулахгүй тул АЮУЛГҮЙ (OpenSC
    /// `get_external_key_retries`). GET CHALLENGE → case-1 `00 82 01 80|kid`
    /// → SW=63Cx бол x нь үлдсэн оролдлого.
    public func externalKeyRetries(kid: UInt8, card: TKSmartCard) async throws -> UInt8? {
        _ = try await getChallenge(card: card)
        let r = try await transmitWithSM(cla: 0x00, ins: 0x82, p1: 0x01, p2: 0x80 | kid,
                                         data: nil, le: nil, card: card)
        lastRetriesSW = r.status
        let sw = r.status
        guard (sw >> 8) == 0x63, (sw & 0xF0) == 0xC0 else { return nil }
        return UInt8(sw & 0x0F)
    }

    /// Оношилгоо: externalKeyRetries-ийн сүүлийн SW.
    public private(set) var lastRetriesSW: UInt16 = 0

    /// PIN баталгаажуулалт — EnterSafe-ийн external authenticate (OpenSC `external_key_auth`).
    /// Энэ карт ЭНГИЙН `00 20` VERIFY-г ХҮЛЭЭЖ АВДАГГҮЙ (бүх reference дээр SW=9403).
    ///
    /// Алгоритм: GET CHALLENGE(8) → key = SHA1(PIN) ‖ 0x00×4 (24 байт, 3-key 3DES) →
    /// 3DES-CBC(iv=0) -ээр challenge-ийг шифрлэж `00 82 01 80|kid` -аар илгээнэ.
    /// Оношилгоо: сүүлийн PIN auth-ийн challenge/cryptogram (offline тулгахад).
    public private(set) var lastPINChallenge: [UInt8] = []
    public private(set) var lastPINCryptogram: [UInt8] = []

    public func externalKeyAuth(pin: String, kid: UInt8, card: TKSmartCard) async throws {
        let challenge = try await getChallenge(card: card)
        guard challenge.count >= 8 else { throw TokenError.authFailed }
        let key = Self.externalAuthKey(pin: pin)
        let cryptogram = try SecureMessaging.des3EncryptCBCPublic(
            key: key, iv: [UInt8](repeating: 0, count: 8), data: Array(challenge.prefix(8)))
        lastPINChallenge = Array(challenge.prefix(8))
        lastPINCryptogram = Array(cryptogram.prefix(8))
        let r = try await transmitWithSM(cla: 0x00, ins: 0x82, p1: 0x01, p2: 0x80 | kid,
                                         data: Array(cryptogram.prefix(8)), card: card)
        guard r.isSuccess else { throw TokenError.unexpectedStatus(r.status) }
        logger.info("PIN баталгаажлаа (kid=\(kid))")
    }

    /// External auth-ийн 3-key 3DES түлхүүр (OpenSC `hash_data(..., SHA1)`).
    ///
    /// Бодит Windows нэвтрэлтийн PC/SC трэйсээс криптограмыг задлан таарууллаа:
    ///   `key = SHA1(pin ASCII, ДҮҮРГЭЛТГҮЙ) ‖ BE32(pin.count)` = 20 + 4 = **24 байт**.
    /// Түлхүүрийг `3DES-EDE3-CBC(iv=0)`-ээр GET CHALLENGE-ийн 8 байтад тавьж илгээнэ.
    ///
    /// (!) OpenSC-ийн `internal_sanitize_pin_info` дэх `pad_length=16` нь ЭНЭ hash-д
    /// хамаарахгүй (тэр нь секрет түлхүүр update хийх өөр зам); external auth-ийн hash нь
    /// PIN-ийг ДҮҮРГЭХГҮЙ, урт нь ЖИНХЭНЭ тэмдэгтийн тоо (8, 16 биш).
    static func externalAuthKey(pin: String) -> [UInt8] {
        let bytes = [UInt8](pin.utf8)
        var key = [UInt8](Insecure.SHA1.hash(data: Data(bytes)))
        let n = UInt32(bytes.count)
        key.append(contentsOf: [UInt8(n >> 24), UInt8((n >> 16) & 0xFF), UInt8((n >> 8) & 0xFF), UInt8(n & 0xFF)])
        return key
    }

    // MARK: - Гарын үсэг (PSO) — ESPK токен, PC/SC трэйсээс

    /// PSO гарын үсэг зурна (SM доор). Windows нэвтрэлтийн трэйсээс задарсан дараалал:
    ///   MSE:SET `00 22 41 B6`  data = 80 01 82 · 81 02 A0 20  (algo 0x82, key 0xA020)
    ///   PSO HASH `00 2A 90 A0` data = 90 20 <SHA-256(32)>      (хэшийг ачаална)
    ///   PSO SIGN `00 2A 9E 9A` le = 0                          → RSA-2048 гарын үсэг (256B)
    ///
    /// `mse`/`hashPrefix` нь трэйсийн ЯГ байтыг оноох боломж (механизм батлахад).
    /// Оношилгоо: signPSO алхмуудын SW.
    public var signTrace: [String] = []

    /// Оношилгоо: зөвхөн PSO HASH илгээж, картын буцаасан digest-ийг авна.
    public func psoHashRaw(hashTLV: [UInt8], card: TKSmartCard) async throws -> [UInt8] {
        _ = try await transmitWithSM(cla: 0x00, ins: 0x22, p1: 0x41, p2: 0xB6,
                                     data: [0x80,0x01,0x82,0x81,0x02,0xA0,0x20], card: card)
        let r = try await transmitWithSM(cla: 0x00, ins: 0x2A, p1: 0x90, p2: 0xA0,
                                         data: hashTLV, le: 0x00, card: card)
        return r.data ?? []
    }

    /// Бүтэн signable message-ийг (=ДАН-ий P payload) токеноор гарын үсэг зурна (ESPK токен).
    /// Карт SHA-256-ийг ӨӨРӨӨ finalize хийдэг тул digest-ийг БИШ, БҮТЭН message-ийг өгнө —
    /// PSO HASH-ийн `81/90/80` streaming ачааллыг `EsignPSOHash.hashTLV` угсарна.
    public func signMessage(_ message: [UInt8], card: TKSmartCard) async throws -> [UInt8] {
        try await signPSO(digest: Array(SHA256.hash(data: Data(message))), card: card,
                          hashTLV: EsignPSOHash.hashTLV(for: message))
    }

    public func signPSO(digest: [UInt8], card: TKSmartCard,
                        mseData: [UInt8] = [0x80, 0x01, 0x82, 0x81, 0x02, 0xA0, 0x20],
                        hashTLV: [UInt8]? = nil) async throws -> [UInt8] {
        precondition(digest.count == 32 || hashTLV != nil, "SHA-256 digest (32 байт)")
        // MSE:SET — түлхүүр + алгоритм сонгоно.
        let mse = try await transmitWithSM(cla: 0x00, ins: 0x22, p1: 0x41, p2: 0xB6,
                                           data: mseData, card: card)
        signTrace.append(String(format: "MSE %04X", mse.status))
        guard mse.isSuccess else { throw TokenError.unexpectedStatus(mse.status) }

        // PSO HASH — SHA-256 хэшийг картанд ачаална (90 20 <hash>).
        let hData = hashTLV ?? ([0x90, 0x20] + digest)
        // Трэйс дээр PSO HASH нь case-4 (хариу буцаадаг) тул Le TLV заавал (le=0x00).
        let ph = try await transmitWithSM(cla: 0x00, ins: 0x2A, p1: 0x90, p2: 0xA0,
                                          data: hData, le: 0x00, card: card)
        signTrace.append(String(format: "HASH %04X", ph.status))
        guard ph.isSuccess else { throw TokenError.unexpectedStatus(ph.status) }

        // PSO SIGN — ачаалсан хэш дээр гарын үсэг.
        let sig = try await transmitWithSM(cla: 0x00, ins: 0x2A, p1: 0x9E, p2: 0x9A,
                                           data: nil, le: 0x00, card: card)
        guard sig.isSuccess, let out = sig.data, !out.isEmpty else {
            throw TokenError.signFailed("PSO SIGN SW=\(sig.hexStatus)")
        }
        return out
    }

    /// Оношилгоонд: FID-ээр SELECT (нийтийн боодол).
    public func selectPublic(_ fid: UInt16, card: TKSmartCard) async throws {
        try await selectByFID(fid, card: card)
    }

    private func selectByFID(_ fid: UInt16, card: TKSmartCard) async throws {
        // le: 0 → SM-д Le TLV (97 01 00) үүснэ. SELECT нь FCI буцаадаг case-4 команд тул
        // OpenSC ч Le TLV-г үргэлж хавсаргадаг; үгүй бол карт SW=6A80 өгнө.
        let resp = try await transmitWithSM(
            cla: 0x00, ins: 0xA4, p1: 0x00, p2: 0x00,
            data: [UInt8(fid >> 8), UInt8(fid & 0xFF)], le: 0x00, card: card)
        guard resp.isSuccess else { throw TokenError.unexpectedStatus(resp.status) }
    }

    /// Сонгогдсон EF-ийг бүтнээр нь уншина (00 B0, 0xFE байтын хэсгүүдээр).
    private func readBinaryFully(card: TKSmartCard) async throws -> [UInt8] {
        var result = [UInt8]()
        var offset: UInt16 = 0
        let chunk: UInt8 = 0xD8   // OpenSC: max_recv_size
        // (!) БОГИНО chunk дээр ЗОГСОХГҮЙ. SM-ийн дор карт дуудлага тутамд хүссэнээс
        // цөөн байт өгч болно; өмнө нь `data.count < chunk` дээр зогсдог байсан тул EF
        // дундуур тасарч, гэрчилгээ бүтэн уншигдахгүй байв (24E0: 1364 байтын гэрчилгээ
        // тасарч, толгой доторх хуурамч SEQUENCE л үлддэг). Хоосон хариу эсвэл алдааны
        // SW (6B00 = EOF) хүртэл үргэлжилнэ.
        while result.count < 8192 {
            let r = try await transmitWithSM(cla: 0x00, ins: 0xB0,
                                             p1: UInt8(offset >> 8), p2: UInt8(offset & 0xFF),
                                             le: chunk, card: card)
            // SW1=0x6C — "Le буруу, зөв урт нь SW2". EF-ийн ҮЛДСЭН хэсэг chunk-аас бага
            // үлдэхэд карт үүнийг өгдөг (жишээ: 2581 байтын EF дээр 11 удаа 216 уншаад
            // SW=6CCD → үлдсэн 205). Урьд нь энд зогсдог байсан тул EF тасарч, 24E0-ийн
            // гэрчилгээ бүтэн уншигдахгүй байв.
            if (r.status >> 8) == 0x6C {
                let exact = UInt8(r.status & 0xFF)
                guard exact > 0 else { break }
                let r2 = try await transmitWithSM(cla: 0x00, ins: 0xB0,
                                                  p1: UInt8(offset >> 8), p2: UInt8(offset & 0xFF),
                                                  le: exact, card: card)
                if let d2 = r2.data, !d2.isEmpty { result.append(contentsOf: d2) }
                break
            }
            // Сүүлийн хэсэгт карт EOF анхааруулга (6282) өгөөд өгөгдлөө хамт буцааж болно.
            if let data = r.data, !data.isEmpty {
                result.append(contentsOf: data)
                offset = offset &+ UInt16(data.count)
            }
            guard r.isSuccess, let d = r.data, !d.isEmpty else { break }
        }
        return result
    }

    /// EF-ийн түүхий агуулгаас X.509 DER-ийг сугална: эхний `30 82 LL LL`-ийг олж,
    /// зарласан уртаар нь таслана (толгой/дүүргэлтийг хаяна).
    static func extractDER(_ raw: [UInt8]) -> [UInt8]? {
        guard raw.count > 8 else { return nil }
        for i in 0..<max(0, raw.count - 8) where raw[i] == 0x30 && raw[i + 1] == 0x82 {
            let len = Int(raw[i + 2]) << 8 | Int(raw[i + 3])
            let total = 4 + len
            guard total > 64, i + total <= raw.count else { continue }
            // X.509 Certificate ::= SEQUENCE { tbsCertificate SEQUENCE, … } тул гадна
            // SEQUENCE-ийн ДАРАА шууд бас SEQUENCE (0x30) ирнэ. Энэ шалгалтгүй бол EF-ийн
            // толгой дотор санамсаргүй тохиосон `30 82` дээр зогсож, задрахгүй хог өгдөг
            // (бодит жишээ: EF 24E0 → 828 байтын хуурамч "гэрчилгээ").
            guard raw[i + 4] == 0x30 else { continue }
            return Array(raw[i..<(i + total)])
        }
        return nil
    }

    // MARK: - Write Certificate

    /// Сертификат токен руу бичих.
    /// APDU: 00 D6 [offset_hi] [offset_lo] [data]
    public func writeCertificate(data: [UInt8], fileID: UInt16 = 0x2441, card: TKSmartCard) async throws {
        guard sm?.isEstablished == true else {
            throw TokenError.authFailed
        }

        // SELECT file
        let selectData: [UInt8] = [UInt8(fileID >> 8), UInt8(fileID & 0xFF)]
        let selectResp = try await transmitWithSM(
            cla: 0x00, ins: 0xA4, p1: 0x00, p2: 0x00,
            data: selectData, card: card
        )
        guard selectResp.isSuccess else {
            throw TokenError.certificateImportFailed
        }

        // Write binary in chunks
        var offset: UInt16 = 0
        let chunkSize = 240

        while Int(offset) < data.count {
            let end = min(Int(offset) + chunkSize, data.count)
            let chunk = Array(data[Int(offset)..<end])

            let response = try await transmitWithSM(
                cla: 0x00, ins: 0xD6,
                p1: UInt8(offset >> 8), p2: UInt8(offset & 0xFF),
                data: chunk, card: card
            )

            guard response.isSuccess else {
                logger.error("WRITE CERT failed at offset \(offset): SW=\(response.hexStatus)")
                throw TokenError.certificateImportFailed
            }

            offset += UInt16(chunk.count)
        }

        logger.info("Certificate бичигдлээ (\(data.count) bytes, fileID=\(String(format: "%04X", fileID)))")
    }
}

// MARK: - Token Info Model

public struct TokenInfo {
    public let atr: [UInt8]
    public let label: String
    public let isFIPS: Bool
    public let appletSelected: Bool

    public var atrHex: String { bytesToHex(atr) }
}
