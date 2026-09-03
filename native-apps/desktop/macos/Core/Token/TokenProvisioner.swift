import Foundation
import GeregeTokenKit
import os

/// Токен тохируулах — GeregeTokenKit ашиглан APDU-аар шууд харилцана.
/// OpenSC CLI хэрэггүй.
actor TokenProvisioner {

    private let kit = GeregeTokenKit.TokenManager.shared
    private let driver = BioPassDriver()
    private let logger = Logger(subsystem: "mn.gerege.nexus.desktop", category: "TokenProvisioner")

    // MARK: - Token мэдээлэл

    func getTokenInfo() async throws -> GeregeTokenKit.TokenInfo {
        try await kit.withSession { [driver] card in
            try await driver.getTokenInfo(card: card)
        }
    }

    // MARK: - Connection test

    func testConnection() async throws -> Bool {
        let challenge = try await kit.withSession { [driver] card in
            _ = try await driver.selectApplet(card: card)
            return try await driver.getChallenge(card: card)
        }
        logger.info("Challenge: \(bytesToHex(challenge))")
        return true
    }

    // MARK: - PIN verify

    func verifyPIN(_ pin: String) async throws {
        try await kit.withSession { [driver] card in
            _ = try await driver.selectApplet(card: card)
            try await driver.verifyPIN(pin, card: card)
        }
    }

    // MARK: - Secure Messaging Session

    /// SM session тогтоож mutual auth хийнэ.
    func establishSecureSession() async throws {
        try await kit.withSession { [driver] card in
            try await driver.establishSecureSession(card: card)
        }
        logger.info("SM session тогтоогдлоо")
    }

    // MARK: - Phase 2: Token Initialize (erase + filesystem + PIN/PUK)

    /// Токен бүрэн initialize — erase + transport keys + filesystem + PINs.
    /// Энэ нь OpenSC `pkcs15-init --create-pkcs15 --store-pin`-ын үйлдэл.
    func initialize(pin: String, puk: String) async throws {
        try await kit.withSession { [driver] card in
            // 1. SM session тогтоох (mutual auth)
            try await driver.establishSecureSession(card: card)
            logger.info("SM session амжилттай — token init эхэлж байна")

            // 2. Erase + transport keys + filesystem + PINs (бүгд дарааллаар)
            try await driver.initializePIN(pin: pin, puk: puk, card: card)
            logger.info("Token initialized — filesystem + PIN/PUK тогтоогдлоо")
        }
    }

    /// Токен бүх хэрэглэгчийн data-г устгана. Factory reset-ийн эхний алхам.
    func eraseCard() async throws {
        try await kit.withSession { [driver] card in
            try await driver.establishSecureSession(card: card)
            try await driver.eraseCard(card: card)
            logger.info("Card erased")
        }
    }

    /// PKCS#15 filesystem бүтэц үүсгэх (MF + App DF + EFs).
    /// Transport key install хийсний дараа дуудна.
    func createFilesystem() async throws {
        try await kit.withSession { [driver] card in
            try await driver.establishSecureSession(card: card)
            try await driver.createPKCS15Filesystem(card: card)
            logger.info("PKCS#15 filesystem үүсгэгдлэа")
        }
    }

    // MARK: - Phase 3: Key Generation

    /// EC key pair үүсгэх — SM session + PIN verify хийсний дараа.
    func generateKeyPair(pin: String) async throws {
        try await kit.withSession { [driver] card in
            // 1. SM session тогтоох
            try await driver.establishSecureSession(card: card)

            // 2. PIN verify
            try await driver.verifyPIN(pin, card: card)
            logger.info("PIN verified — key generation эхэлж байна")

            // 3. EC key pair үүсгэх
            try await driver.generateECKeyPair(card: card)
            logger.info("EC key pair амжилттай үүсгэгдлэа")
        }
    }

    /// RSA key pair үүсгэх.
    func generateRSAKeyPair(pin: String, keySize: Int = 2048) async throws {
        try await kit.withSession { [driver] card in
            try await driver.establishSecureSession(card: card)
            try await driver.verifyPIN(pin, card: card)
            try await driver.generateRSAKeyPair(keySize: keySize, card: card)
            logger.info("RSA key pair амжилттай үүсгэгдлэа")
        }
    }

    // MARK: - Certificate Operations

    /// Токеноос сертификат уншиx.
    func readCertificate() async throws -> [UInt8] {
        try await kit.withSession { [driver] card in
            _ = try await driver.selectApplet(card: card)
            return try await driver.readCertificate(card: card)
        }
    }

    /// Сертификат токен руу бичих (SM шаардлагатай).
    func writeCertificate(data: [UInt8], pin: String) async throws {
        try await kit.withSession { [driver] card in
            try await driver.establishSecureSession(card: card)
            try await driver.verifyPIN(pin, card: card)
            try await driver.writeCertificate(data: data, card: card)
            logger.info("Certificate амжилттай бичигдлэа")
        }
    }

    /// Токен дээрх БҮХ гэрчилгээг APDU-аар уншина + алхам бүрийн МӨРДЛӨГӨ.
    ///
    /// Мөрдлөгө нь заавал: карт бүр өөр байршуулалттай (ESPK / FIPS / SM шаардсан) тул
    /// "олдсонгүй" гэдэг нь ямар алхам дээр яагаад тасарсныг хэлж чадах ёстой.
    func readCertificatesTraced(pin: String) async -> (certs: [(fileID: UInt16, der: [UInt8])], trace: [String]) {
        var trace: [String] = []

        // 1) EnterSafe-ESPK (Tridium / Gerege Signature): MF → DF 2003 → EF 24xx.
        //    SM ч, PIN ч шаардахгүй.
        do {
            let (espk, smOK) = try await kit.withSession { [driver] card -> ([(fileID: UInt16, der: [UInt8])], Bool) in
                let certs = try await driver.readESPKCertificates(card: card)
                return (certs, driver.sm?.isEstablished == true)
            }
            trace.append("ESPK(2003): \(espk.count) гэрчилгээ, SM=\(smOK ? "тийм" : "үгүй")")
            if !espk.isEmpty { return (espk, trace) }
        } catch {
            trace.append("ESPK(2003): \(error.localizedDescription)")
        }

        // 2) Гэрэгэ-гийн FIPS applet — applet сонгоод EF 2441+.
        do {
            let plain = try await kit.withSession { [driver] card in
                _ = try await driver.selectApplet(card: card)
                return try await driver.readAllCertificates(card: card)
            }
            trace.append("FIPS applet: \(plain.count) гэрчилгээ")
            if !plain.isEmpty { return (plain, trace) }
        } catch {
            trace.append("FIPS applet: \(error.localizedDescription)")
        }

        // 3) SM (+ PIN) шаардсан карт.
        do {
            let sm = try await kit.withSession { [driver] card in
                try await driver.establishSecureSession(card: card)
                if !pin.isEmpty { try await driver.verifyPIN(pin, card: card) }
                return try await driver.readAllCertificates(card: card)
            }
            trace.append("SM\(pin.isEmpty ? "" : "+PIN"): \(sm.count) гэрчилгээ")
            return (sm, trace)
        } catch {
            trace.append("SM\(pin.isEmpty ? "" : "+PIN"): \(error.localizedDescription)")
        }
        return ([], trace)
    }

    /// Оношилгоо: гэрчилгээ олдохгүй үед картын CDF (Certificate Directory File, 0x4404)
    /// болон label EF (0x2440)-ийн түүхий байтыг уншина. CDF дотор гэрчилгээний бодит
    /// EF ID-ууд бичээстэй байдаг тул сканнердах мужийг эндээс тодруулна.
    func readDiagnosticFiles() async -> [(fileID: UInt16, bytes: [UInt8])] {
        let ids: [UInt16] = [0x4404, 0x2440, 0x5031, 0x5032]
        return (try? await kit.withSession { [driver] card in
            _ = try? await driver.selectApplet(card: card)
            var out: [(UInt16, [UInt8])] = []
            for fid in ids {
                if let raw = try? await driver.readCertificate(fileID: fid, card: card), !raw.isEmpty {
                    out.append((fid, Array(raw.prefix(64))))
                }
            }
            return out
        }) ?? []
    }

    func isOpenSCInstalled() -> Bool {
        true // GeregeTokenKit ашиглах тул OpenSC хэрэггүй
    }

    // MARK: - PKCS#11 full provision (FEITIAN middleware)

    /// FEITIAN Castle PKCS#11-ээр токен factory reset хийж, PIN + EC keypair тогтооно.
    /// i386 AdminMgr ажиллахгүй тул энэ нь Apple Silicon дээрх provision path.
    /// ⚠ Токен дээрх өмнөх бүх түлхүүр устана.
    func fullProvisionViaPKCS11(soPIN: String, userPIN: String,
                                 label: String = "Gerege",
                                 keyLabel: String = "gerege-sign") async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.fullProvision(
                soPIN: soPIN, userPIN: userPIN,
                label: label, keyLabel: keyLabel,
                keyID: [0x01]
            )
        }.value
        logger.info("PKCS#11 full provision амжилттай")
    }

    // MARK: - PKCS#11 PIN management (FEITIAN middleware)

    /// User PIN солих — хуучин PIN-ээр нэвтэрч шинэ PIN тогтооно.
    func changeUserPIN(oldPIN: String, newPIN: String) async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.changeUserPIN(oldPIN: oldPIN, newPIN: newPIN)
        }.value
        logger.info("User PIN changed via PKCS#11")
    }

    /// SO (admin) PIN солих.
    func changeSOPIN(oldPIN: String, newPIN: String) async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.changeSOPIN(oldPIN: oldPIN, newPIN: newPIN)
        }.value
        logger.info("SO PIN changed via PKCS#11")
    }

    /// Locked болсон User PIN-г SO PIN-ээр тайлж шинэ PIN тогтооно.
    func unlockUserPIN(soPIN: String, newUserPIN: String) async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.unlockUserPIN(soPIN: soPIN, newUserPIN: newUserPIN)
        }.value
        logger.info("User PIN unlocked via PKCS#11")
    }

    // MARK: - PKCS#11 object management (M-T6)

    /// Токен дээрх бүх объектыг (cert / key) жагсаана.
    /// Токен дээрх объектууд. Windows клиенттэй ижил зарчим: суусан БҮХ PKCS#11 модулиар
    /// дамжуулж үзээд үр дүнг нэгтгэнэ — нэг модуль картыг танихгүй байх нь бусад нь ч
    /// танихгүй гэсэн үг БИШ (OpenSC нь Tridium/MonPass зэрэг PKCS#15 токеныг, вендорын
    /// middleware нь зөвхөн өөрийн картаа таьдаг). Бүгд унасан үед л алдаа шиднэ.
    func listTokenObjects(pin: String) async throws -> [GeregeTokenKit.PKCS11Module.TokenObjectInfo] {
        try await Task.detached(priority: .userInitiated) { [logger] in
            let modules = GeregeTokenKit.PKCS11Module.openAll()
            guard !modules.isEmpty else {
                // Нэг ч модуль ачаалагдаагүй — юу хийхийг зааж өгсөн алдаа.
                _ = try GeregeTokenKit.PKCS11Module.open()
                return []
            }
            var out: [GeregeTokenKit.PKCS11Module.TokenObjectInfo] = []
            var lastError: Error?
            for (path, module) in modules {
                do {
                    let objs = try module.listObjects(pin: pin)
                    logger.info("PKCS#11 [\(path, privacy: .public)]: \(objs.count) объект")
                    out.append(contentsOf: objs)
                } catch {
                    // Тухайн модуль энэ картыг танихгүй/слотгүй — дараагийнхаар үргэлжилнэ.
                    logger.info("PKCS#11 [\(path, privacy: .public)] алгасав: \(error.localizedDescription, privacy: .public)")
                    lastError = error
                }
            }
            if out.isEmpty, let lastError { throw lastError }
            return out
        }.value
    }

    /// DER X.509 сертификатыг токен руу бичих (PKCS#11).
    func importCertificatePKCS11(pin: String, label: String, certificateDER: [UInt8]) async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.importCertificate(pin: pin, label: label, certificateDER: certificateDER)
        }.value
        logger.info("Certificate imported via PKCS#11 (label=\(label))")
    }

    /// Токен дээрх объектыг id + төрлөөр устгах.
    func deleteTokenObject(pin: String, idHex: String, kind: String) async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.deleteObject(pin: pin, idHex: idHex, kind: kind)
        }.value
        logger.info("Token object deleted (id=\(idHex), kind=\(kind))")
    }

    /// EC P-256 гарын үсгийн түлхүүр хос үүсгэх (PKCS#11).
    func generateSigningKeyPKCS11(pin: String, label: String = "gerege-sign") async throws {
        try await Task.detached(priority: .userInitiated) {
            let module = try GeregeTokenKit.PKCS11Module.open()
            try module.generateSigningKey(pin: pin, label: label)
        }.value
        logger.info("Signing keypair generated via PKCS#11")
    }

    // MARK: - CSR (PKCS#10) generation + submit

    /// PKCS#11 дамжин токен дээрх EC P-256 pubkey-ээр CSR үүсгэнэ.
    /// Subject CN = nationalID.
    func generateCSR(pin: String, nationalID: String, name: String,
                      organization: String? = nil, keyLabel: String? = nil) async throws -> String {
        let module = try GeregeTokenKit.PKCS11Module.open()
        let subject = GeregeTokenKit.CSR.Subject(
            commonName: nationalID,
            organization: organization ?? name,
            country: "MN"
        )
        let (_, pem) = try await module.generateCSR(
            pin: pin, keyLabel: keyLabel, subject: subject
        )
        logger.info("CSR үүсгэгдлэа (\(pem.count) bytes PEM)")
        return pem
    }

}
