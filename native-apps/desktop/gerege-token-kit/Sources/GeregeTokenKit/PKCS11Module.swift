import Foundation
import os

/// PKCS#11 v2.40 minimal wrapper.
/// FEITIAN Castle FTSmartToken `libcastle.1.0.0.dylib` (ARM64 + x86_64).
/// Токен PIN-аар нэвтэрч түлхүүрээр гарын үсэг зурдаг standard interface.
///
/// Ашиглах:
/// ```swift
/// let p11 = try PKCS11Module.open()
/// try p11.initialize()
/// let slots = try p11.getSlotList()
/// let session = try p11.openSession(slotId: slots[0])
/// try p11.login(session: session, pin: "12345678")
/// let sig = try p11.signECDSA(session: session, keyLabel: "gerege", hash: hash)
/// ```
public final class PKCS11Module {

    // MARK: - PKCS#11 Type Aliases (macOS 64-bit)

    public typealias CK_RV = UInt
    public typealias CK_ULONG = UInt
    public typealias CK_SLOT_ID = UInt
    public typealias CK_SESSION_HANDLE = UInt
    public typealias CK_OBJECT_HANDLE = UInt
    public typealias CK_FLAGS = UInt
    public typealias CK_ATTRIBUTE_TYPE = UInt
    public typealias CK_MECHANISM_TYPE = UInt
    public typealias CK_USER_TYPE = UInt

    // MARK: - Return codes (PKCS#11 v2.40)

    public static let CKR_OK: CK_RV                    = 0x00000000
    public static let CKR_PIN_INCORRECT: CK_RV         = 0x000000A0
    public static let CKR_PIN_LOCKED: CK_RV            = 0x000000A4
    public static let CKR_SESSION_HANDLE_INVALID: CK_RV = 0x000000B3
    public static let CKR_USER_ALREADY_LOGGED_IN: CK_RV = 0x00000100
    public static let CKR_USER_NOT_LOGGED_IN: CK_RV    = 0x00000101
    public static let CKR_CRYPTOKI_NOT_INITIALIZED: CK_RV = 0x00000190

    // MARK: - User types

    public static let CKU_USER: CK_USER_TYPE = 1
    public static let CKU_SO: CK_USER_TYPE = 0

    // MARK: - Session flags

    public static let CKF_SERIAL_SESSION: CK_FLAGS = 0x00000004
    public static let CKF_RW_SESSION: CK_FLAGS = 0x00000002

    // MARK: - Mechanism types

    public static let CKM_ECDSA: CK_MECHANISM_TYPE = 0x00001041
    public static let CKM_EC_KEY_PAIR_GEN: CK_MECHANISM_TYPE = 0x00001040
    public static let CKM_RSA_PKCS: CK_MECHANISM_TYPE = 0x00000001
    public static let CKM_RSA_PKCS_KEY_PAIR_GEN: CK_MECHANISM_TYPE = 0x00000000
    public static let CKM_SHA256_RSA_PKCS: CK_MECHANISM_TYPE = 0x00000040

    // MARK: - Key types

    public static let CKK_EC: CK_ULONG = 0x00000003
    public static let CKK_RSA: CK_ULONG = 0x00000000

    // MARK: - Object classes

    public static let CKO_PRIVATE_KEY: CK_ULONG = 0x00000003
    public static let CKO_PUBLIC_KEY: CK_ULONG = 0x00000002
    public static let CKO_CERTIFICATE: CK_ULONG = 0x00000001

    // MARK: - Certificate types

    public static let CKC_X_509: CK_ULONG = 0x00000000

    // MARK: - Attribute types

    public static let CKA_CLASS: CK_ATTRIBUTE_TYPE = 0x00000000
    public static let CKA_TOKEN: CK_ATTRIBUTE_TYPE = 0x00000001
    public static let CKA_PRIVATE: CK_ATTRIBUTE_TYPE = 0x00000002
    public static let CKA_LABEL: CK_ATTRIBUTE_TYPE = 0x00000003
    public static let CKA_KEY_TYPE: CK_ATTRIBUTE_TYPE = 0x00000100
    public static let CKA_ID: CK_ATTRIBUTE_TYPE = 0x00000102
    public static let CKA_SENSITIVE: CK_ATTRIBUTE_TYPE = 0x00000103
    public static let CKA_SIGN: CK_ATTRIBUTE_TYPE = 0x00000108
    public static let CKA_VERIFY: CK_ATTRIBUTE_TYPE = 0x0000010A
    public static let CKA_VALUE: CK_ATTRIBUTE_TYPE = 0x00000011
    public static let CKA_CERTIFICATE_TYPE: CK_ATTRIBUTE_TYPE = 0x00000080
    public static let CKA_SUBJECT: CK_ATTRIBUTE_TYPE = 0x00000101
    public static let CKA_EC_PARAMS: CK_ATTRIBUTE_TYPE = 0x00000180
    public static let CKA_EC_POINT: CK_ATTRIBUTE_TYPE = 0x00000181
    public static let CKA_MODULUS: CK_ATTRIBUTE_TYPE = 0x00000120
    public static let CKA_EXTRACTABLE: CK_ATTRIBUTE_TYPE = 0x00000162

    // MARK: - DER OIDs for EC curves (CKA_EC_PARAMS value)

    /// secp256r1 (P-256): OID 1.2.840.10045.3.1.7 — DER encoded
    public static let ecParamsP256: [UInt8] = [
        0x06, 0x08, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07
    ]

    // MARK: - C Structs

    public struct CK_ATTRIBUTE {
        var type: CK_ATTRIBUTE_TYPE
        var pValue: UnsafeMutableRawPointer?
        var ulValueLen: CK_ULONG
    }

    public struct CK_MECHANISM {
        var mechanism: CK_MECHANISM_TYPE
        var pParameter: UnsafeMutableRawPointer?
        var ulParameterLen: CK_ULONG
    }

    // MARK: - Errors

    public enum P11Error: LocalizedError {
        case libraryNotFound(String)
        case symbolNotFound(String)
        /// C_GetSlotList(tokenPresent) хоосон буцаав. Энэ нь PKCS#11 АЛДАА БИШ (rv = CKR_OK):
        /// уншигч байгаа ч энэ модуль картыг токен гэж танихгүй байна. Урьд нь
        /// `ckError("getSlotList (empty)", 0)` гэж шидээд UI дээр "амжилтгүй: CKR=0x00000000"
        /// (= CKR_OK) гэсэн зөрчилтэй мессеж гардаг байв.
        case noTokenPresent
        case ckError(String, CK_RV)

        public var errorDescription: String? {
            switch self {
            case .noTokenPresent:
                return "Уншигч дээр карт байна, гэхдээ ачаалагдсан PKCS#11 модулиудын аль нь "
                    + "ч түүнийг токен гэж танихгүй байна. Ихэвчлэн: карт хараахан "
                    + "тохируулагдаагүй (PKCS#15 бүтэцгүй), эсвэл тухайн вендорын middleware "
                    + "суугаагүй. Тохиргоо → PKCS#11 модулиас замыг нь зааж өгч болно."
            case .libraryNotFound(let tried):
                return "PKCS#11 модуль олдсонгүй. macOS дээр Windows-ийн CSP/minidriver шиг "
                    + "автомат гүүр байдаггүй тул токены объект уншихад middleware суусан байх "
                    + "шаардлагатай — хамгийн хялбар нь Терминалд `brew install opensc`. "
                    + "Өөр зам ашиглаж байгаа бол Тохиргоо → PKCS#11 модулиас зааж өгнө үү. "
                    + "Үзсэн замууд: \(tried)"
            case .symbolNotFound(let name):    return "PKCS#11 function олдсонгүй: \(name)"
            case .ckError(let fn, let rv):     return String(format: "%@ амжилтгүй: CKR=0x%08X", fn, rv)
            }
        }
    }

    // MARK: - State

    private let handle: UnsafeMutableRawPointer
    private let logger = Logger(subsystem: "mn.gerege.token-kit", category: "PKCS11")
    private var initialized = false

    // Function pointers (C ABI)
    private typealias FnInitialize = @convention(c) (UnsafeMutableRawPointer?) -> CK_RV
    private typealias FnFinalize = @convention(c) (UnsafeMutableRawPointer?) -> CK_RV
    private typealias FnGetSlotList = @convention(c) (UInt8, UnsafeMutablePointer<CK_SLOT_ID>?, UnsafeMutablePointer<CK_ULONG>) -> CK_RV
    private typealias FnOpenSession = @convention(c) (CK_SLOT_ID, CK_FLAGS, UnsafeMutableRawPointer?, UnsafeMutableRawPointer?, UnsafeMutablePointer<CK_SESSION_HANDLE>) -> CK_RV
    private typealias FnCloseSession = @convention(c) (CK_SESSION_HANDLE) -> CK_RV
    private typealias FnLogin = @convention(c) (CK_SESSION_HANDLE, CK_USER_TYPE, UnsafePointer<UInt8>?, CK_ULONG) -> CK_RV
    private typealias FnLogout = @convention(c) (CK_SESSION_HANDLE) -> CK_RV
    // For struct pointers use UnsafeMutableRawPointer — @convention(c) can't
    // carry Swift struct pointers directly; we cast inside the wrappers.
    private typealias FnFindObjectsInit = @convention(c) (CK_SESSION_HANDLE, UnsafeMutableRawPointer?, CK_ULONG) -> CK_RV
    private typealias FnFindObjects = @convention(c) (CK_SESSION_HANDLE, UnsafeMutablePointer<CK_OBJECT_HANDLE>?, CK_ULONG, UnsafeMutablePointer<CK_ULONG>) -> CK_RV
    private typealias FnFindObjectsFinal = @convention(c) (CK_SESSION_HANDLE) -> CK_RV
    private typealias FnGetAttributeValue = @convention(c) (CK_SESSION_HANDLE, CK_OBJECT_HANDLE, UnsafeMutableRawPointer?, CK_ULONG) -> CK_RV
    private typealias FnSignInit = @convention(c) (CK_SESSION_HANDLE, UnsafeMutableRawPointer, CK_OBJECT_HANDLE) -> CK_RV
    private typealias FnSign = @convention(c) (CK_SESSION_HANDLE, UnsafePointer<UInt8>?, CK_ULONG, UnsafeMutablePointer<UInt8>?, UnsafeMutablePointer<CK_ULONG>) -> CK_RV
    // Provisioning — C_InitToken, C_InitPIN, C_SetPIN, C_GenerateKeyPair, C_CreateObject
    private typealias FnInitToken = @convention(c) (CK_SLOT_ID, UnsafePointer<UInt8>?, CK_ULONG, UnsafePointer<UInt8>?) -> CK_RV
    private typealias FnInitPIN = @convention(c) (CK_SESSION_HANDLE, UnsafePointer<UInt8>?, CK_ULONG) -> CK_RV
    private typealias FnSetPIN = @convention(c) (CK_SESSION_HANDLE, UnsafePointer<UInt8>?, CK_ULONG, UnsafePointer<UInt8>?, CK_ULONG) -> CK_RV
    private typealias FnGenerateKeyPair = @convention(c) (CK_SESSION_HANDLE, UnsafeMutableRawPointer, UnsafeMutableRawPointer?, CK_ULONG, UnsafeMutableRawPointer?, CK_ULONG, UnsafeMutablePointer<CK_OBJECT_HANDLE>, UnsafeMutablePointer<CK_OBJECT_HANDLE>) -> CK_RV
    private typealias FnCreateObject = @convention(c) (CK_SESSION_HANDLE, UnsafeMutableRawPointer?, CK_ULONG, UnsafeMutablePointer<CK_OBJECT_HANDLE>) -> CK_RV
    private typealias FnDestroyObject = @convention(c) (CK_SESSION_HANDLE, CK_OBJECT_HANDLE) -> CK_RV

    private let fnInitialize: FnInitialize
    private let fnFinalize: FnFinalize
    private let fnGetSlotList: FnGetSlotList
    private let fnOpenSession: FnOpenSession
    private let fnCloseSession: FnCloseSession
    private let fnLogin: FnLogin
    private let fnLogout: FnLogout
    private let fnFindObjectsInit: FnFindObjectsInit
    private let fnFindObjects: FnFindObjects
    private let fnFindObjectsFinal: FnFindObjectsFinal
    private let fnGetAttributeValue: FnGetAttributeValue
    private let fnSignInit: FnSignInit
    private let fnSign: FnSign
    private let fnInitToken: FnInitToken
    private let fnInitPIN: FnInitPIN
    private let fnSetPIN: FnSetPIN
    private let fnGenerateKeyPair: FnGenerateKeyPair
    private let fnCreateObject: FnCreateObject
    /// Optional: not every middleware exports C_DestroyObject. A missing symbol
    /// must not break `open()`, so this is resolved leniently.
    private let fnDestroyObject: FnDestroyObject?

    // MARK: - Default library paths

    /// Нэр дэвшигч PKCS#11 модулиуд. Windows клиентийн `Pkcs11ModuleCandidates`-ийн macOS
    /// хувилбар: НЭГ зам биш ЖАГСААЛТ, учир нь карт бүрийг өөр модуль л таньдаг (OpenSC нь
    /// PIV/PKCS#15/GIDS, FEITIAN-ийн castle нь ePass2003/BioPass гэх мэт). Дараалал чухал —
    /// эхний ачаалагдсан нь ялна.
    ///
    /// (!) Windows дээр ийм асуудал гардаггүй нь middleware суулгахад CSP/minidriver нь
    /// системийн гэрчилгээний санд объектуудыг ӨӨРӨӨ нийтэлдэг (CNG backend). macOS-д
    /// ийм автомат гүүр БАЙХГҮЙ: карт нь CryptoTokenKit драйвертай (ж: PIV) биш бол
    /// PKCS#11 модуль ЗААВАЛ суусан байх ёстой. Хамгийн хялбар нь `brew install opensc`.
    public static let defaultLibraryPaths = [
        // ── 1. Ерөнхий зориулалтын модуль — ГУРАВДАГЧ ЭТГЭЭДИЙН токен унших гол зам ──
        // OpenSC нь PKCS#15/PIV/GIDS картуудыг таньдаг тул Tridium, MonPass зэрэг бусад
        // үндэсний ҮГ-ийн токенууд үүгээр нээгддэг. Homebrew (Apple Silicon → Intel) →
        // албан ёсны .pkg суулгац гэсэн дарааллаар.
        "/opt/homebrew/lib/opensc-pkcs11.so",
        "/opt/homebrew/lib/pkcs11/opensc-pkcs11.so",
        "/usr/local/lib/opensc-pkcs11.so",
        "/usr/local/lib/pkcs11/opensc-pkcs11.so",
        "/Library/OpenSC/lib/opensc-pkcs11.so",
        // SafeNet / Gemalto eToken — банкны токенуудад тохиолддог вендорын middleware.
        "/usr/local/lib/libeToken.dylib",

        // ── 2. FEITIAN castle — ГЭРЭГЭ-ийн ӨӨРИЙН токен (BioPass/ePass2003) ──
        // Энэ токеныг апп ердийн үедээ APDU-гаар (BioPassDriver, Secure Messaging) шууд
        // удирддаг тул PKCS#11 нь зөвхөн НЭМЭЛТ зам: вендорын middleware сууж байвал
        // объектуудыг түүгээр ч харж болно. Жагсаалтын СҮҮЛД — гуравдагч этгээдийн
        // картыг эхлээд ерөнхий модулиар үзэх ёстой.
        "/usr/local/lib/libcastle.1.0.0.dylib",
        "/usr/local/lib/libcastle.dylib",
    ]

    /// `defaultLibraryPaths` дээр нэмээд хэрэглэгчийн заасан зам (Тохиргоо → PKCS#11 модуль)
    /// болон `GEREGE_PKCS11_MODULE` орчны хувьсагчийг ЭХЭЛЖ үзнэ — Windows дахь
    /// `Pkcs11ModulePath` override-той ижил үүрэг.
    public static let modulePathOverrideKey = "token.pkcs11ModulePath"

    /// Бодит дараалал: override → default жагсаалт.
    public static var candidateLibraryPaths: [String] {
        var out: [String] = []
        if let o = UserDefaults.standard.string(forKey: modulePathOverrideKey),
           !o.trimmingCharacters(in: .whitespaces).isEmpty {
            out.append(o.trimmingCharacters(in: .whitespaces))
        }
        if let e = ProcessInfo.processInfo.environment["GEREGE_PKCS11_MODULE"], !e.isEmpty {
            out.append(e)
        }
        out.append(contentsOf: defaultLibraryPaths)
        return out
    }

    /// Файл систем дээр БОДИТООР байгаа нэр дэвшигчид (диагностик/Тохиргооны UI-д).
    public static var installedLibraryPaths: [String] {
        candidateLibraryPaths.filter { FileManager.default.fileExists(atPath: $0) }
    }

    // MARK: - Open

    /// PKCS#11 dynamic library-г ачаалан C_* function-уудыг dlsym-аар олох.
    public static func open(libraryPath: String? = nil) throws -> PKCS11Module {
        let candidates: [String]
        if let p = libraryPath { candidates = [p] }
        else { candidates = candidateLibraryPaths }

        var handle: UnsafeMutableRawPointer?
        for path in candidates {
            handle = dlopen(path, Self.dlopenMode)
            if handle != nil { break }
        }
        guard let h = handle else {
            // Аль ч зам ажиллаагүй — хэрэглэгчид ЯМАР зам үзсэнээ бус, ЮУ хийхээ хэлнэ
            // (нэг зам хэвлэх нь "яг тэр файлыг тавь" гэсэн буруу дохио өгдөг байв).
            throw P11Error.libraryNotFound(candidates.joined(separator: ", "))
        }
        return try PKCS11Module(handle: h)
    }

    /// Ачаалагдаж БАЙГАА бүх нэр дэвшигч модулийг нээнэ (Windows клиент дэх "модуль тутамд
    /// нэг Pkcs11TokenProvider"-ийн эквивалент).
    ///
    /// ЯАГААД: модуль бүр өөр өөр картыг л таньдаг — OpenSC нь PKCS#15/PIV (Tridium, MonPass
    /// зэрэг гуравдагч ҮГ-ийн токен), вендорын middleware нь зөвхөн өөрийн картаа. Ганц
    /// модулиар зогсвол эхнийх нь ачаалагдаад картыг танихгүй үед "токен алга" гэж БУРУУ
    /// дүгнэнэ. Windows дээр яг үүнээс болж модуль тутамд provider бүртгэдэг.
    ///
    /// Дараалал `candidateLibraryPaths`-тай ижил; давхардсан замыг нэг л удаа нээнэ.
    public static func openAll() -> [(path: String, module: PKCS11Module)] {
        var out: [(String, PKCS11Module)] = []
        var seen = Set<String>()
        for path in candidateLibraryPaths {
            // Давхардлыг ЗАМААР биш ЗАНГИЛААГААР шалгана: homebrew-гийн
            // `lib/opensc-pkcs11.so` ба `lib/pkcs11/opensc-pkcs11.so` бол НЭГ файл.
            // Хоёр зам гэж үзвэл нэг санг хоёр удаа C_Initialize/C_Finalize хийж,
            // PKCS#11-ийн дүрэм зөрчигдөж, UI дээр нэг backend хоёр мөр болно.
            guard seen.insert((path as NSString).resolvingSymlinksInPath).inserted else { continue }
            guard let h = dlopen(path, dlopenMode) else { continue }
            guard let m = try? PKCS11Module(handle: h) else { continue }
            out.append((path, m))
        }
        return out
    }

    /// Модулийг ХЭЗЭЭ Ч буулгахгүй (`RTLD_NODELETE`).
    ///
    /// Middleware нь `C_Finalize` дотроосоо ажлаа `dispatch_async`-аар хойшлуулдаг
    /// (PC/SC-ийн таслалт, өөрийн дотоод thread-ийн зогсолт). `dlclose` нь код болон
    /// мөрүүдийг нь ТЭР ДОЛООХОН АГШИНД санах ойгоос салгадаг тул хойшилсон блок
    /// ажиллах үедээ буугаагүй хаяг уншиж `EXC_BAD_ACCESS` (`_platform_strcmp`) өгнө —
    /// Токен уншигч дэлгэц нээх бүрд яг тэр болж байв. Crash-ийн стек дээр
    /// тэмдэггүй түүхий хаяг харагдаж байсан нь санг нь буулгасны шинж.
    ///
    /// Тиймээс NSS, p11-kit, OpenSC-ийн өөрийн хэрэглүүрүүд ч модулиа буулгадаггүй:
    /// PKCS#11 модуль бол процессийн туршид үлдэх зүйл. Дахин ачаалах шаардлагагүй —
    /// `C_Initialize`/`C_Finalize` мөчлөгийг нэг ачаалагдсан сан дээр давтаж болно.
    static let dlopenMode = RTLD_NOW | RTLD_NODELETE

    private init(handle: UnsafeMutableRawPointer) throws {
        self.handle = handle

        func resolve<T>(_ name: String, _: T.Type) throws -> T {
            guard let sym = dlsym(handle, name) else {
                throw P11Error.symbolNotFound(name)
            }
            return unsafeBitCast(sym, to: T.self)
        }

        fnInitialize        = try resolve("C_Initialize", FnInitialize.self)
        fnFinalize          = try resolve("C_Finalize", FnFinalize.self)
        fnGetSlotList       = try resolve("C_GetSlotList", FnGetSlotList.self)
        fnOpenSession       = try resolve("C_OpenSession", FnOpenSession.self)
        fnCloseSession      = try resolve("C_CloseSession", FnCloseSession.self)
        fnLogin             = try resolve("C_Login", FnLogin.self)
        fnLogout            = try resolve("C_Logout", FnLogout.self)
        fnFindObjectsInit   = try resolve("C_FindObjectsInit", FnFindObjectsInit.self)
        fnFindObjects       = try resolve("C_FindObjects", FnFindObjects.self)
        fnFindObjectsFinal  = try resolve("C_FindObjectsFinal", FnFindObjectsFinal.self)
        fnGetAttributeValue = try resolve("C_GetAttributeValue", FnGetAttributeValue.self)
        fnSignInit          = try resolve("C_SignInit", FnSignInit.self)
        fnSign              = try resolve("C_Sign", FnSign.self)
        fnInitToken         = try resolve("C_InitToken", FnInitToken.self)
        fnInitPIN           = try resolve("C_InitPIN", FnInitPIN.self)
        fnSetPIN            = try resolve("C_SetPIN", FnSetPIN.self)
        fnGenerateKeyPair   = try resolve("C_GenerateKeyPair", FnGenerateKeyPair.self)
        fnCreateObject      = try resolve("C_CreateObject", FnCreateObject.self)
        // Lenient — keep the module usable even if the middleware lacks it.
        fnDestroyObject     = dlsym(handle, "C_DestroyObject").map {
            unsafeBitCast($0, to: FnDestroyObject.self)
        }
    }

    deinit {
        if initialized { _ = fnFinalize(nil) }
        // `dlclose` ЗОРИУДААР байхгүй — `dlopenMode`-ийн тайлбарыг үз. Сан нь
        // `RTLD_NODELETE`-ээр ачаалагдсан тул энд хаалаа ч буухгүй, гэхдээ
        // дуудахгүй байгаа нь санааг ил болгоно.
    }

    // MARK: - Lifecycle

    public func initialize() throws {
        let rv = fnInitialize(nil)
        // PKCS#11 allows ALREADY_INITIALIZED (0x191) as OK
        if rv != Self.CKR_OK && rv != 0x00000191 {
            throw P11Error.ckError("C_Initialize", rv)
        }
        initialized = true
    }

    public func finalize() {
        if initialized {
            _ = fnFinalize(nil)
            initialized = false
        }
    }

    // MARK: - Slots & Sessions

    /// Холбогдсон бүх slot-ын ID буцаана (tokenPresent=true бол зөвхөн токен залгасан slot-ууд).
    public func getSlotList(tokenPresent: Bool = true) throws -> [CK_SLOT_ID] {
        var count: CK_ULONG = 0
        let rv1 = fnGetSlotList(tokenPresent ? 1 : 0, nil, &count)
        guard rv1 == Self.CKR_OK else { throw P11Error.ckError("C_GetSlotList", rv1) }
        if count == 0 { return [] }

        var slots = [CK_SLOT_ID](repeating: 0, count: Int(count))
        let rv2 = slots.withUnsafeMutableBufferPointer { buf in
            fnGetSlotList(tokenPresent ? 1 : 0, buf.baseAddress, &count)
        }
        guard rv2 == Self.CKR_OK else { throw P11Error.ckError("C_GetSlotList", rv2) }
        return Array(slots.prefix(Int(count)))
    }

    public func openSession(slotId: CK_SLOT_ID, readWrite: Bool = false) throws -> CK_SESSION_HANDLE {
        var session: CK_SESSION_HANDLE = 0
        let flags = Self.CKF_SERIAL_SESSION | (readWrite ? Self.CKF_RW_SESSION : 0)
        let rv = fnOpenSession(slotId, flags, nil, nil, &session)
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_OpenSession", rv) }
        return session
    }

    public func closeSession(_ session: CK_SESSION_HANDLE) {
        _ = fnCloseSession(session)
    }

    // MARK: - Login / Logout

    public func login(session: CK_SESSION_HANDLE, pin: String, userType: CK_USER_TYPE = CKU_USER) throws {
        var pinBytes = Array(pin.utf8)
        defer { for i in 0..<pinBytes.count { pinBytes[i] = 0 } }

        let rv = pinBytes.withUnsafeBufferPointer { buf in
            fnLogin(session, userType, buf.baseAddress, CK_ULONG(buf.count))
        }
        // Already logged in is OK
        if rv == Self.CKR_OK || rv == Self.CKR_USER_ALREADY_LOGGED_IN { return }
        throw P11Error.ckError("C_Login", rv)
    }

    public func logout(_ session: CK_SESSION_HANDLE) {
        _ = fnLogout(session)
    }

    // MARK: - Find Objects

    /// Attribute template-ээр object хайна.
    public func findObjects(session: CK_SESSION_HANDLE,
                             template: [CK_ATTRIBUTE],
                             maxCount: Int = 16) throws -> [CK_OBJECT_HANDLE] {
        var mutableTemplate = template
        let rvInit = mutableTemplate.withUnsafeMutableBufferPointer { buf in
            fnFindObjectsInit(session, UnsafeMutableRawPointer(buf.baseAddress), CK_ULONG(buf.count))
        }
        guard rvInit == Self.CKR_OK else { throw P11Error.ckError("C_FindObjectsInit", rvInit) }

        defer { _ = fnFindObjectsFinal(session) }

        var handles = [CK_OBJECT_HANDLE](repeating: 0, count: maxCount)
        var found: CK_ULONG = 0
        let rv = handles.withUnsafeMutableBufferPointer { buf in
            fnFindObjects(session, buf.baseAddress, CK_ULONG(buf.count), &found)
        }
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_FindObjects", rv) }
        return Array(handles.prefix(Int(found)))
    }

    /// Label-аар private key олох.
    public func findPrivateKey(session: CK_SESSION_HANDLE, label: String? = nil) throws -> CK_OBJECT_HANDLE? {
        var objClass = Self.CKO_PRIVATE_KEY
        let classPtr = withUnsafeMutablePointer(to: &objClass) { UnsafeMutableRawPointer($0) }
        var template: [CK_ATTRIBUTE] = [
            CK_ATTRIBUTE(type: Self.CKA_CLASS, pValue: classPtr,
                         ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size))
        ]

        if let label = label {
            var labelBytes = Array(label.utf8)
            let labelPtr = labelBytes.withUnsafeMutableBufferPointer { UnsafeMutableRawPointer($0.baseAddress) }
            template.append(CK_ATTRIBUTE(type: Self.CKA_LABEL, pValue: labelPtr,
                                          ulValueLen: CK_ULONG(labelBytes.count)))
            return try findObjects(session: session, template: template, maxCount: 1).first
        }
        return try findObjects(session: session, template: template, maxCount: 1).first
    }

    // MARK: - Sign

    /// ECDSA гарын үсэг. Hash нь raw (SHA-256 32 байт).
    /// Буцаах утга: r||s (64 байт P-256 дээр).
    public func signECDSA(session: CK_SESSION_HANDLE,
                           privateKey: CK_OBJECT_HANDLE,
                           hash: [UInt8]) throws -> [UInt8] {
        var mechanism = CK_MECHANISM(mechanism: Self.CKM_ECDSA,
                                      pParameter: nil, ulParameterLen: 0)
        let rvInit = withUnsafeMutablePointer(to: &mechanism) { mPtr in
            fnSignInit(session, UnsafeMutableRawPointer(mPtr), privateKey)
        }
        guard rvInit == Self.CKR_OK else { throw P11Error.ckError("C_SignInit", rvInit) }

        // Length probe
        var sigLen: CK_ULONG = 0
        let rv1 = hash.withUnsafeBufferPointer { hBuf in
            fnSign(session, hBuf.baseAddress, CK_ULONG(hBuf.count), nil, &sigLen)
        }
        guard rv1 == Self.CKR_OK else { throw P11Error.ckError("C_Sign(probe)", rv1) }

        var signature = [UInt8](repeating: 0, count: Int(sigLen))
        let rv2 = hash.withUnsafeBufferPointer { hBuf in
            signature.withUnsafeMutableBufferPointer { sBuf in
                fnSign(session, hBuf.baseAddress, CK_ULONG(hBuf.count), sBuf.baseAddress, &sigLen)
            }
        }
        guard rv2 == Self.CKR_OK else { throw P11Error.ckError("C_Sign", rv2) }
        return Array(signature.prefix(Int(sigLen)))
    }

    // MARK: - Read EC public key

    /// Public key object-оос CKA_EC_POINT attribute уншина.
    /// Буцаах утга: DER OCTET STRING-ийн дотоод (raw EC point, `04||X||Y` эсвэл compressed).
    public func readECPoint(session: CK_SESSION_HANDLE,
                              publicKey: CK_OBJECT_HANDLE) throws -> [UInt8] {
        // 1. Length probe
        var probe = CK_ATTRIBUTE(type: Self.CKA_EC_POINT, pValue: nil, ulValueLen: 0)
        let rv1 = withUnsafeMutablePointer(to: &probe) { ptr in
            fnGetAttributeValue(session, publicKey, UnsafeMutableRawPointer(ptr), 1)
        }
        guard rv1 == Self.CKR_OK, probe.ulValueLen > 0 else {
            throw P11Error.ckError("C_GetAttributeValue(probe EC_POINT)", rv1)
        }

        // 2. Allocate + fetch
        let size = Int(probe.ulValueLen)
        var buffer = [UInt8](repeating: 0, count: size)
        let rv2 = buffer.withUnsafeMutableBufferPointer { buf in
            var attr = CK_ATTRIBUTE(type: Self.CKA_EC_POINT,
                                     pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                     ulValueLen: CK_ULONG(size))
            return withUnsafeMutablePointer(to: &attr) { ptr in
                fnGetAttributeValue(session, publicKey, UnsafeMutableRawPointer(ptr), 1)
            }
        }
        guard rv2 == Self.CKR_OK else {
            throw P11Error.ckError("C_GetAttributeValue(EC_POINT)", rv2)
        }

        // CKA_EC_POINT нь DER-encoded OCTET STRING — дотоодыг нь унших.
        // 04 [len] [data] формат.
        if buffer.count > 2 && buffer[0] == 0x04 {
            let len = Int(buffer[1])
            if len < 0x80 && buffer.count >= 2 + len {
                return Array(buffer[2..<(2 + len)])
            }
            // Long form length
            if buffer[1] == 0x81 && buffer.count > 2 {
                let l = Int(buffer[2])
                if buffer.count >= 3 + l { return Array(buffer[3..<(3 + l)]) }
            }
        }
        // Fallback — raw
        return buffer
    }

    /// Label-аар public key олох.
    public func findPublicKey(session: CK_SESSION_HANDLE, label: String? = nil) throws -> CK_OBJECT_HANDLE? {
        var objClass = Self.CKO_PUBLIC_KEY
        let classPtr = withUnsafeMutablePointer(to: &objClass) { UnsafeMutableRawPointer($0) }
        var template: [CK_ATTRIBUTE] = [
            CK_ATTRIBUTE(type: Self.CKA_CLASS, pValue: classPtr,
                         ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size))
        ]
        if let label = label {
            var labelBytes = Array(label.utf8)
            let labelPtr = labelBytes.withUnsafeMutableBufferPointer { UnsafeMutableRawPointer($0.baseAddress) }
            template.append(CK_ATTRIBUTE(type: Self.CKA_LABEL, pValue: labelPtr,
                                          ulValueLen: CK_ULONG(labelBytes.count)))
        }
        return try findObjects(session: session, template: template, maxCount: 1).first
    }

    // MARK: - Token Initialization (C_InitToken / C_InitPIN / C_SetPIN)

    /// Factory reset токен + SO PIN тогтоох.
    /// Label 32-bytes зайнд padding.
    /// ⚠ Бүх түлхүүр, сертификат устана.
    public func initToken(slotId: CK_SLOT_ID, soPIN: String, label: String) throws {
        var soBytes = Array(soPIN.utf8)
        defer { for i in 0..<soBytes.count { soBytes[i] = 0 } }

        // PKCS#11 spec: label нь 32 байт, зай (0x20)-аар padding
        var labelBytes = [UInt8](repeating: 0x20, count: 32)
        let nameBytes = Array(label.utf8.prefix(32))
        for i in 0..<nameBytes.count { labelBytes[i] = nameBytes[i] }

        let rv = soBytes.withUnsafeBufferPointer { soBuf in
            labelBytes.withUnsafeBufferPointer { lblBuf in
                fnInitToken(slotId, soBuf.baseAddress, CK_ULONG(soBuf.count), lblBuf.baseAddress)
            }
        }
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_InitToken", rv) }
        logger.info("Token initialized (slot=\(slotId), label=\(label))")
    }

    /// SO session-д нэвтэрсний дараа User PIN тогтоох.
    public func initPIN(session: CK_SESSION_HANDLE, userPIN: String) throws {
        var pinBytes = Array(userPIN.utf8)
        defer { for i in 0..<pinBytes.count { pinBytes[i] = 0 } }

        let rv = pinBytes.withUnsafeBufferPointer { buf in
            fnInitPIN(session, buf.baseAddress, CK_ULONG(buf.count))
        }
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_InitPIN", rv) }
        logger.info("User PIN initialized")
    }

    /// User PIN солих (login хийсний дараа).
    public func setPIN(session: CK_SESSION_HANDLE, oldPIN: String, newPIN: String) throws {
        var oldBytes = Array(oldPIN.utf8)
        var newBytes = Array(newPIN.utf8)
        defer {
            for i in 0..<oldBytes.count { oldBytes[i] = 0 }
            for i in 0..<newBytes.count { newBytes[i] = 0 }
        }

        let rv = oldBytes.withUnsafeBufferPointer { oBuf in
            newBytes.withUnsafeBufferPointer { nBuf in
                fnSetPIN(session, oBuf.baseAddress, CK_ULONG(oBuf.count),
                          nBuf.baseAddress, CK_ULONG(nBuf.count))
            }
        }
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_SetPIN", rv) }
        logger.info("PIN changed")
    }

    // MARK: - Key Pair Generation (C_GenerateKeyPair)

    /// EC P-256 keypair токен дээр үүсгэх.
    /// Session нь R/W + User login хийгдсэн байх.
    public func generateECKeyPair(session: CK_SESSION_HANDLE,
                                    label: String,
                                    keyID: [UInt8]) throws -> (publicKey: CK_OBJECT_HANDLE,
                                                                privateKey: CK_OBJECT_HANDLE) {
        var mechanism = CK_MECHANISM(mechanism: Self.CKM_EC_KEY_PAIR_GEN,
                                      pParameter: nil, ulParameterLen: 0)

        // Public key template
        var pubClass: CK_ULONG = Self.CKO_PUBLIC_KEY
        var pubKeyType: CK_ULONG = Self.CKK_EC
        var ckTrue: UInt8 = 0x01
        var ecParams = Self.ecParamsP256
        var labelBytes = Array(label.utf8)
        var idBytes = keyID

        var pubTemplate = [CK_ATTRIBUTE]()
        withUnsafeMutablePointer(to: &pubClass) { p in
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_CLASS, pValue: UnsafeMutableRawPointer(p),
                                             ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size)))
        }
        withUnsafeMutablePointer(to: &pubKeyType) { p in
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_KEY_TYPE, pValue: UnsafeMutableRawPointer(p),
                                             ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size)))
        }
        withUnsafeMutablePointer(to: &ckTrue) { p in
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_TOKEN, pValue: UnsafeMutableRawPointer(p),
                                             ulValueLen: 1))
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_VERIFY, pValue: UnsafeMutableRawPointer(p),
                                             ulValueLen: 1))
        }
        ecParams.withUnsafeMutableBufferPointer { buf in
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_EC_PARAMS,
                                             pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                             ulValueLen: CK_ULONG(buf.count)))
        }
        labelBytes.withUnsafeMutableBufferPointer { buf in
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_LABEL,
                                             pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                             ulValueLen: CK_ULONG(buf.count)))
        }
        idBytes.withUnsafeMutableBufferPointer { buf in
            pubTemplate.append(CK_ATTRIBUTE(type: Self.CKA_ID,
                                             pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                             ulValueLen: CK_ULONG(buf.count)))
        }

        // Private key template
        var privClass: CK_ULONG = Self.CKO_PRIVATE_KEY
        var privKeyType: CK_ULONG = Self.CKK_EC
        var privTemplate = [CK_ATTRIBUTE]()
        withUnsafeMutablePointer(to: &privClass) { p in
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_CLASS, pValue: UnsafeMutableRawPointer(p),
                                              ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size)))
        }
        withUnsafeMutablePointer(to: &privKeyType) { p in
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_KEY_TYPE, pValue: UnsafeMutableRawPointer(p),
                                              ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size)))
        }
        withUnsafeMutablePointer(to: &ckTrue) { p in
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_TOKEN, pValue: UnsafeMutableRawPointer(p),
                                              ulValueLen: 1))
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_PRIVATE, pValue: UnsafeMutableRawPointer(p),
                                              ulValueLen: 1))
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_SIGN, pValue: UnsafeMutableRawPointer(p),
                                              ulValueLen: 1))
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_SENSITIVE, pValue: UnsafeMutableRawPointer(p),
                                              ulValueLen: 1))
        }
        labelBytes.withUnsafeMutableBufferPointer { buf in
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_LABEL,
                                              pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                              ulValueLen: CK_ULONG(buf.count)))
        }
        idBytes.withUnsafeMutableBufferPointer { buf in
            privTemplate.append(CK_ATTRIBUTE(type: Self.CKA_ID,
                                              pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                              ulValueLen: CK_ULONG(buf.count)))
        }

        var pubHandle: CK_OBJECT_HANDLE = 0
        var privHandle: CK_OBJECT_HANDLE = 0

        let rv = withUnsafeMutablePointer(to: &mechanism) { mPtr in
            pubTemplate.withUnsafeMutableBufferPointer { pubBuf in
                privTemplate.withUnsafeMutableBufferPointer { privBuf in
                    fnGenerateKeyPair(session,
                                       UnsafeMutableRawPointer(mPtr),
                                       UnsafeMutableRawPointer(pubBuf.baseAddress),
                                       CK_ULONG(pubBuf.count),
                                       UnsafeMutableRawPointer(privBuf.baseAddress),
                                       CK_ULONG(privBuf.count),
                                       &pubHandle, &privHandle)
                }
            }
        }
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_GenerateKeyPair", rv) }
        logger.info("EC keypair generated (label=\(label))")
        return (pubHandle, privHandle)
    }

    // MARK: - Create Object (certificate upload)

    /// X.509 сертификатыг токен дээр хадгалах.
    /// Session нь R/W + User login хийгдсэн байх.
    public func writeCertificate(session: CK_SESSION_HANDLE,
                                  certificateDER: [UInt8],
                                  label: String,
                                  keyID: [UInt8],
                                  subjectDER: [UInt8]? = nil) throws -> CK_OBJECT_HANDLE {
        var objClass: CK_ULONG = Self.CKO_CERTIFICATE
        var certType: CK_ULONG = Self.CKC_X_509
        var ckTrue: UInt8 = 0x01
        var certBytes = certificateDER
        var labelBytes = Array(label.utf8)
        var idBytes = keyID
        var subjectBytes = subjectDER ?? [0x30, 0x00]  // empty SEQUENCE if none

        var template = [CK_ATTRIBUTE]()
        withUnsafeMutablePointer(to: &objClass) { p in
            template.append(CK_ATTRIBUTE(type: Self.CKA_CLASS, pValue: UnsafeMutableRawPointer(p),
                                          ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size)))
        }
        withUnsafeMutablePointer(to: &certType) { p in
            template.append(CK_ATTRIBUTE(type: Self.CKA_CERTIFICATE_TYPE, pValue: UnsafeMutableRawPointer(p),
                                          ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size)))
        }
        withUnsafeMutablePointer(to: &ckTrue) { p in
            template.append(CK_ATTRIBUTE(type: Self.CKA_TOKEN, pValue: UnsafeMutableRawPointer(p),
                                          ulValueLen: 1))
        }
        certBytes.withUnsafeMutableBufferPointer { buf in
            template.append(CK_ATTRIBUTE(type: Self.CKA_VALUE,
                                          pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                          ulValueLen: CK_ULONG(buf.count)))
        }
        subjectBytes.withUnsafeMutableBufferPointer { buf in
            template.append(CK_ATTRIBUTE(type: Self.CKA_SUBJECT,
                                          pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                          ulValueLen: CK_ULONG(buf.count)))
        }
        labelBytes.withUnsafeMutableBufferPointer { buf in
            template.append(CK_ATTRIBUTE(type: Self.CKA_LABEL,
                                          pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                          ulValueLen: CK_ULONG(buf.count)))
        }
        idBytes.withUnsafeMutableBufferPointer { buf in
            template.append(CK_ATTRIBUTE(type: Self.CKA_ID,
                                          pValue: UnsafeMutableRawPointer(buf.baseAddress),
                                          ulValueLen: CK_ULONG(buf.count)))
        }

        var handle: CK_OBJECT_HANDLE = 0
        let rv = template.withUnsafeMutableBufferPointer { buf in
            fnCreateObject(session, UnsafeMutableRawPointer(buf.baseAddress),
                            CK_ULONG(buf.count), &handle)
        }
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_CreateObject(cert)", rv) }
        logger.info("Certificate written (\(certificateDER.count)B, label=\(label))")
        return handle
    }

    // MARK: - High-level helpers

    /// Хамгийн энгийн signECDSA: slot сонгож, PIN-аар нэвтэрч, label-аар түлхүүр олж гарын үсэг зурна.
    /// Session автоматаар хаагдана.
    public func signECDSA(pin: String, keyLabel: String? = nil, hash: [UInt8]) throws -> [UInt8] {
        try initialize()
        defer { finalize() }

        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else {
            throw P11Error.noTokenPresent
        }

        let session = try openSession(slotId: slot)
        defer { closeSession(session) }

        try login(session: session, pin: pin)
        defer { logout(session) }

        guard let key = try findPrivateKey(session: session, label: keyLabel) else {
            throw P11Error.ckError("findPrivateKey (not found)", 0)
        }

        return try signECDSA(session: session, privateKey: key, hash: hash)
    }

    /// Токен бүрэн provision — factory reset + SO PIN + User PIN + EC keypair үүсгэх.
    /// Дуусахад токен-д keypair бий (label-ээр олдоно), CSR үүсгэхэд бэлэн.
    /// ⚠ Токен дээрх өмнөх бүх түлхүүр, сертификат устана.
    public func fullProvision(soPIN: String, userPIN: String,
                               label: String, keyLabel: String, keyID: [UInt8]) throws {
        try initialize()
        defer { finalize() }

        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else {
            throw P11Error.noTokenPresent
        }

        // Step 1: C_InitToken (factory reset + SO PIN)
        try initToken(slotId: slot, soPIN: soPIN, label: label)

        // Step 2: R/W session + login as SO + initPIN
        let session = try openSession(slotId: slot, readWrite: true)
        defer { closeSession(session) }

        try login(session: session, pin: soPIN, userType: Self.CKU_SO)
        try initPIN(session: session, userPIN: userPIN)
        logout(session)

        // Step 3: Login as user + generate EC keypair
        try login(session: session, pin: userPIN)
        _ = try generateECKeyPair(session: session, label: keyLabel, keyID: keyID)
        logout(session)

        logger.info("Full provision амжилттай — EC keypair бий болсон")
    }

    /// Token-оос EC P-256 pubkey уншаад CSR үүсгэнэ.
    /// Signer нь token дээрх private key-ээр hash-г зурна.
    public func generateCSR(pin: String, keyLabel: String? = nil,
                             subject: CSR.Subject) async throws -> (der: [UInt8], pem: String) {
        try initialize()
        defer { finalize() }

        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else {
            throw P11Error.noTokenPresent
        }
        let session = try openSession(slotId: slot)
        defer { closeSession(session) }

        try login(session: session, pin: pin)
        defer { logout(session) }

        guard let pub = try findPublicKey(session: session, label: keyLabel) else {
            throw P11Error.ckError("findPublicKey (not found)", 0)
        }
        guard let priv = try findPrivateKey(session: session, label: keyLabel) else {
            throw P11Error.ckError("findPrivateKey (not found)", 0)
        }

        let ecPoint = try readECPoint(session: session, publicKey: pub)

        return try await CSR.buildP256(
            subject: subject,
            publicKeyPoint: ecPoint
        ) { [unowned self] hash in
            try self.signECDSA(session: session, privateKey: priv, hash: hash)
        }
    }

    // MARK: - High-level PIN management

    /// User PIN солих — хуучин PIN-ээр нэвтэрч C_SetPIN дуудна.
    /// (PKCS#11: C_SetPIN нь нэвтэрсэн хэрэглэгчийн PIN-г солино.)
    public func changeUserPIN(oldPIN: String, newPIN: String) throws {
        try initialize()
        defer { finalize() }

        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else {
            throw P11Error.noTokenPresent
        }

        let session = try openSession(slotId: slot, readWrite: true)
        defer { closeSession(session) }

        try login(session: session, pin: oldPIN, userType: Self.CKU_USER)
        defer { logout(session) }

        try setPIN(session: session, oldPIN: oldPIN, newPIN: newPIN)
        logger.info("User PIN changed")
    }

    /// SO (admin) PIN солих — хуучин SO PIN-ээр нэвтэрч C_SetPIN дуудна.
    public func changeSOPIN(oldPIN: String, newPIN: String) throws {
        try initialize()
        defer { finalize() }

        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else {
            throw P11Error.noTokenPresent
        }

        let session = try openSession(slotId: slot, readWrite: true)
        defer { closeSession(session) }

        try login(session: session, pin: oldPIN, userType: Self.CKU_SO)
        defer { logout(session) }

        try setPIN(session: session, oldPIN: oldPIN, newPIN: newPIN)
        logger.info("SO PIN changed")
    }

    /// Locked болсон User PIN-г SO PIN-ээр тайлж шинэ User PIN тогтооно.
    /// (SO session-д нэвтэрч C_InitPIN дуудна — retry counter reset болно.)
    public func unlockUserPIN(soPIN: String, newUserPIN: String) throws {
        try initialize()
        defer { finalize() }

        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else {
            throw P11Error.noTokenPresent
        }

        let session = try openSession(slotId: slot, readWrite: true)
        defer { closeSession(session) }

        try login(session: session, pin: soPIN, userType: Self.CKU_SO)
        defer { logout(session) }

        try initPIN(session: session, userPIN: newUserPIN)
        logger.info("User PIN unlocked via SO PIN")
    }

    // MARK: - Object enumeration / management (M-T6)

    /// Metadata for an on-token object (cert / key / data).
    public struct TokenObjectInfo: Sendable, Equatable {
        public let idHex: String
        public let label: String
        public let kind: String      // "certificate" | "privateKey" | "publicKey" | "data"
        public let keyType: String   // "EC" | "RSA" | "" (non-keys)
        public init(idHex: String, label: String, kind: String, keyType: String) {
            self.idHex = idHex; self.label = label; self.kind = kind; self.keyType = keyType
        }
    }

    /// Read a fixed-size CK_ULONG attribute. Returns nil if unreadable.
    private func readULong(session: CK_SESSION_HANDLE, handle: CK_OBJECT_HANDLE,
                           type: CK_ATTRIBUTE_TYPE) -> CK_ULONG? {
        var value: CK_ULONG = 0
        let rv = withUnsafeMutablePointer(to: &value) { vp -> CK_RV in
            var attr = CK_ATTRIBUTE(type: type, pValue: UnsafeMutableRawPointer(vp),
                                    ulValueLen: CK_ULONG(MemoryLayout<CK_ULONG>.size))
            return withUnsafeMutablePointer(to: &attr) {
                fnGetAttributeValue(session, handle, UnsafeMutableRawPointer($0), 1)
            }
        }
        return rv == Self.CKR_OK ? value : nil
    }

    /// Read a variable-length byte attribute (probe length, then fetch).
    private func readBytes(session: CK_SESSION_HANDLE, handle: CK_OBJECT_HANDLE,
                           type: CK_ATTRIBUTE_TYPE) -> [UInt8]? {
        var probe = CK_ATTRIBUTE(type: type, pValue: nil, ulValueLen: 0)
        let rv1 = withUnsafeMutablePointer(to: &probe) {
            fnGetAttributeValue(session, handle, UnsafeMutableRawPointer($0), 1)
        }
        guard rv1 == Self.CKR_OK, probe.ulValueLen > 0, probe.ulValueLen < 65536 else { return nil }
        let size = Int(probe.ulValueLen)
        var buf = [UInt8](repeating: 0, count: size)
        let rv2 = buf.withUnsafeMutableBufferPointer { bp -> CK_RV in
            var attr = CK_ATTRIBUTE(type: type, pValue: UnsafeMutableRawPointer(bp.baseAddress),
                                    ulValueLen: CK_ULONG(size))
            return withUnsafeMutablePointer(to: &attr) {
                fnGetAttributeValue(session, handle, UnsafeMutableRawPointer($0), 1)
            }
        }
        return rv2 == Self.CKR_OK ? buf : nil
    }

    private func kind(forClass cls: CK_ULONG) -> String {
        switch cls {
        case Self.CKO_CERTIFICATE: return "certificate"
        case Self.CKO_PRIVATE_KEY: return "privateKey"
        case Self.CKO_PUBLIC_KEY:  return "publicKey"
        default:                   return "data"
        }
    }

    private static func hex(_ bytes: [UInt8]) -> String {
        bytes.map { String(format: "%02x", $0) }.joined()
    }

    /// Enumerate all token objects in an open (logged-in) session.
    public func listObjects(session: CK_SESSION_HANDLE) throws -> [TokenObjectInfo] {
        let handles = try findObjects(session: session, template: [], maxCount: 64)
        var out: [TokenObjectInfo] = []
        for h in handles {
            let cls = readULong(session: session, handle: h, type: Self.CKA_CLASS) ?? 0xFFFF
            let label = readBytes(session: session, handle: h, type: Self.CKA_LABEL)
                .map { String(decoding: $0, as: UTF8.self) } ?? ""
            let idHex = readBytes(session: session, handle: h, type: Self.CKA_ID)
                .map { Self.hex($0) } ?? ""
            var keyType = ""
            if cls == Self.CKO_PRIVATE_KEY || cls == Self.CKO_PUBLIC_KEY,
               let kt = readULong(session: session, handle: h, type: Self.CKA_KEY_TYPE) {
                keyType = (kt == Self.CKK_EC) ? "EC" : (kt == Self.CKK_RSA ? "RSA" : "")
            }
            out.append(TokenObjectInfo(idHex: idHex, label: label,
                                       kind: kind(forClass: cls), keyType: keyType))
        }
        return out
    }

    /// Destroy a single object handle (requires C_DestroyObject in the middleware).
    public func destroyObject(session: CK_SESSION_HANDLE, handle: CK_OBJECT_HANDLE) throws {
        guard let destroy = fnDestroyObject else {
            throw P11Error.symbolNotFound("C_DestroyObject")
        }
        let rv = destroy(session, handle)
        guard rv == Self.CKR_OK else { throw P11Error.ckError("C_DestroyObject", rv) }
    }

    // MARK: - High-level object helpers

    /// Login and list every object on the token. Session auto-closes.
    public func listObjects(pin: String) throws -> [TokenObjectInfo] {
        try initialize(); defer { finalize() }
        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else { throw P11Error.noTokenPresent }
        let session = try openSession(slotId: slot); defer { closeSession(session) }
        try login(session: session, pin: pin); defer { logout(session) }
        return try listObjects(session: session)
    }

    /// Import a DER X.509 certificate onto the token under `label`.
    public func importCertificate(pin: String, label: String,
                                  certificateDER: [UInt8], keyID: [UInt8] = [0x01]) throws {
        try initialize(); defer { finalize() }
        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else { throw P11Error.noTokenPresent }
        let session = try openSession(slotId: slot, readWrite: true); defer { closeSession(session) }
        try login(session: session, pin: pin); defer { logout(session) }
        _ = try writeCertificate(session: session, certificateDER: certificateDER,
                                 label: label, keyID: keyID)
    }

    /// Delete the object matching `idHex` + `kind` (re-found within the session).
    public func deleteObject(pin: String, idHex: String, kind: String) throws {
        try initialize(); defer { finalize() }
        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else { throw P11Error.noTokenPresent }
        let session = try openSession(slotId: slot, readWrite: true); defer { closeSession(session) }
        try login(session: session, pin: pin); defer { logout(session) }

        let handles = try findObjects(session: session, template: [], maxCount: 64)
        for h in handles {
            let cls = readULong(session: session, handle: h, type: Self.CKA_CLASS) ?? 0xFFFF
            let hid = readBytes(session: session, handle: h, type: Self.CKA_ID)
                .map { Self.hex($0) } ?? ""
            if self.kind(forClass: cls) == kind && hid == idHex {
                try destroyObject(session: session, handle: h)
                logger.info("Destroyed object id=\(idHex) kind=\(kind)")
                return
            }
        }
        throw P11Error.ckError("object_not_found", 0)
    }

    /// Login and generate an EC P-256 signing keypair on the token.
    public func generateSigningKey(pin: String, label: String, keyID: [UInt8] = [0x01]) throws {
        try initialize(); defer { finalize() }
        let slots = try getSlotList(tokenPresent: true)
        guard let slot = slots.first else { throw P11Error.noTokenPresent }
        let session = try openSession(slotId: slot, readWrite: true); defer { closeSession(session) }
        try login(session: session, pin: pin); defer { logout(session) }
        _ = try generateECKeyPair(session: session, label: label, keyID: keyID)
        logger.info("Signing keypair generated (label=\(label))")
    }
}
