import Foundation

/// Web backend-ийн нийтийн `/api/*` route-ууд (Next.js, `web/src/app/api/*`).
///
/// Desktop app нь browser-тэй ЯГ ижил урсгалаар эдгээр first-party route-уудыг
/// дууддаг — RP secret/UUID клиентэд байхгүй, тэдгээрийг web сервер өөрөө
/// Go RP-API (`/v3/*`) руу дамжуулахдаа хэрэглэнэ.
enum Endpoint {

    // ─── Нэвтрэлт — ЭНЭ ПЛАТФОРМЫН RP ─────────────────────────────────────
    //
    // Доорх `/api/*` бүлэг нь eID Mongolia-гийн ӨӨРИЙН вэб аппын route-ууд:
    // nginx тэдгээрийг `eidmongolia.mn` руу proxy хийдэг тул session нь ТЭДНИЙ
    // RP-ээр («RP Demo Bank») үүсч, иргэн утсан дээрээ тэр нэрийг уншдаг байв.
    // Нэвтрэлт нь эндээс хойш платформын өөрийн RP-ээр явна — вэб (`EIDLogin`)
    // яг эдгээр route-уудыг дууддаг тул апп, хөтөч хоёр НЭГ RP, нэг дүрэм.
    //
    // `callbackUrl` нь `${PUBLIC_ORIGIN}/auth/eid/callback`, ХООСОН, эсвэл
    // цөмийн `EID_APP_CALLBACKS`-д бүртгэгдсэн аппын схем байна — өөр бүхэн 400.
    // Ширээн дээр ХООСОН нь санаатай (`AppConfig.appToAppCallback`): зөвшөөрөгч
    // нь өөр төхөөрөмж тул буцах хаяг өгвөл eID апп тэр утсан дээр байхгүй
    // аппыг нээхийг оролдоно. Утсан дээр схемээ илгээнэ.
    /// `POST /api/v1/auth/eid/start` → `{session_id, device_link_url, verification_code}`.
    case authStart(callbackURL: String)
    /// `POST /api/v1/auth/eid/start-id` `{national_id, callbackUrl}` — регистрээр push.
    case authStartByID(nationalID: String, callbackURL: String)
    /// `POST /api/v1/auth/eid/poll` `{session_id}` → `{state, identity}`.
    /// Сервер тал хүсэлтийг 25 секунд хүртэл барина (`eid.PollWindow`).
    case authPoll(sessionID: String)

    // ─── Нэвтрэлт — eID Mongolia-гийн вэб апп (demo RP) ────────────────────
    /// QR session: `POST /api/start` → `{sessionId, qr, deviceLinkBase, vc}`.
    /// QR-д `qr` (= sessionId) утгыг кодлоно, `vc`-г дэлгэцэнд харуулна.
    case start
    /// РД push: `POST /api/login-notify` `{register}` → `{sessionId, vc}`.
    /// `register` нь РД / иргэний дугаар / PNOMN-… аль нь ч байж болно.
    /// Rate limit: 60с-д 3 (429 → rate_limited).
    case loginNotify(register: String)
    /// Session poll: `GET /api/status?sessionId=&pollToken=` — сервер 1с барина,
    /// клиент давтан дуудна. `pollToken` нь start/login-notify хариунд ирдэг —
    /// sessionId нь QR-д ил тул дангаараа PII унших эрх олгохгүй. COMPLETE/OK
    /// үед web нь гарын үсгийг криптограф баталгаажуулж cert subject-оос
    /// name/idNumber-ийг задалж өгнө.
    case status(sessionID: String, pollToken: String)
    /// PDF гарын үсгийн session (web demo хуудастай яг ижил урсгал):
    /// `POST /api/sign-pdf-start {etsi, digestB64, fileName, callbackUrl}` →
    /// `{sessionId, vc, pollToken}`. Digest-ийг клиент өөрөө SHA-256-аар тооцно
    /// (stamp хийх ИЖИЛ байт), утас PIN2-оор энэ digest-ийг зурна.
    /// Rate limit: 60с-д 3 (per etsi, 429 → rate_limited). Poll нь auth-тай
    /// ижил `/api/status?sessionId=&pollToken=`.
    case signPdfStart(etsi: String, digestB64: String, fileName: String)

    // ─── Зөвхөн унших самбар ───────────────────────────────────────────────
    /// `POST /api/representations {sessionId, pollToken}` → төлөөлж чадах байгууллагууд.
    /// `POST /api/children {sessionId, pollToken}` → асран хамгаалж буй хүүхдүүд.
    /// Хоёулаа session-bound: pollToken 10 минут хүчинтэй тул нэвтэрсэн ДАРААХ агшинд
    /// нэг удаа татаж авна (desktop-д бүртгэх/цуцлах үйлдэл БАЙХГҮЙ — утсаар л хийнэ).
    case representations(sessionID: String, pollToken: String)
    case children(sessionID: String, pollToken: String)
    /// `POST /api/dashboard {sessionId, pollToken}` → XYP-ийн хүний хураангуй.
    ///
    /// Энд иргэний нэр МОНГОЛООР ирнэ. Гэрчилгээний subject дэх нэр (`/api/status`-ийн
    /// `name`) нь ЛАТИН галиг — «ERDENEBAT TSENDDORJ» — тул дэлгэц дээр харагдах нэрийг
    /// эндээс авна. Мөн session-bound: representations/children-тэй ижил pollToken.
    case dashboard(sessionID: String, pollToken: String)

    // ─── ESIGN "программ токен" (ДАН/isf.mn — физик USB токенгүй нэвтрэлт) ──
    /// `POST /api/certificates {sessionId, pollToken}` → жагсаалт + `signing`/`auth`
    /// гэрчилгээ base64 DER-ээр. Session-bound (representations/children-тэй ижил):
    /// ДАН-д илгээх payload-д гэрчилгээ ЗУРАХААС ӨМНӨ хэрэгтэй тул нэвтэрсэн даруйд
    /// татаж Keychain-д кэшлэнэ. `personId` хүлээж авдаггүй — тэр нь broken access
    /// control байсныг аудитаар зассан (web/src/lib/sessionIdentity.ts).
    case certificates(sessionID: String, pollToken: String)
    /// `POST /api/esign-sign {personId, digestB64, key, displayText}` →
    /// `{sessionId, vc, pollToken}`. ДУРЫН 32 байт digest-ийг PIN2-оор зуруулна;
    /// гарын үсгийн ТҮҮХИЙ утгыг `/api/status`-ийн `signatureValueB64`-оос авна
    /// (PDF stamp хийхгүй). Rate limit: 60с-д 3 (per personId).
    case esignSign(personID: String, digestB64: String, displayText: String)

    var method: String {
        switch self {
        case .status: return "GET"
        default:      return "POST"
        }
    }

    var path: String {
        switch self {
        case .authStart:      return "/api/v1/auth/eid/start"
        case .authStartByID:  return "/api/v1/auth/eid/start-id"
        case .authPoll:       return "/api/v1/auth/eid/poll"
        case .start:        return "/api/start"
        case .loginNotify:  return "/api/login-notify"
        case .status:       return "/api/status"
        case .signPdfStart: return "/api/sign-pdf-start"
        case .representations: return "/api/representations"
        case .children:        return "/api/children"
        case .certificates:    return "/api/certificates"
        case .dashboard:       return "/api/dashboard"
        case .esignSign:       return "/api/esign-sign"
        }
    }

    var queryItems: [URLQueryItem]? {
        switch self {
        case .status(let id, let token):
            return [
                URLQueryItem(name: "sessionId", value: id),
                URLQueryItem(name: "pollToken", value: token),
            ]
        default:
            return nil
        }
    }

    /// Нэмэлт header БАЙХГҮЙ.
    ///
    /// Өмнө нь `X-Eid-Client: desktop` илгээж, web тал түүгээр session-д зарлах RP-г
    /// сонгодог байв. Одоо платформ дээр RP НЭГ (`rpclient.ts § RP_SELF`): ширээ,
    /// утас, хөтөч гурвуулан энэ платформын ӨӨРИЙН клиент болохоос гуравдагч RP биш.
    /// Тиймээс header нь юуг ч сонгохоо больсон — үлдээвэл юу ч хийдэггүй мөр
    /// хэдэн жил амьдарна.
    var extraHeaders: [String: String] { [:] }

    func body() throws -> Data? {
        switch self {
        case .authStart(let callback):
            return try JSONEncoder().encode(["callbackUrl": callback])
        case .authStartByID(let nationalID, let callback):
            return try JSONEncoder().encode(["national_id": nationalID, "callbackUrl": callback])
        case .authPoll(let sessionID):
            return try JSONEncoder().encode(["session_id": sessionID])
        case .start:
            // App2App буцалт: ширээн дээр хоосон, утсан дээр өөрийн схем
            // (`AppConfig.appToAppCallback`). Сервер тал үүнийг RP-ийн
            // allowlist-аар шалгаж session дээр хадгална; eID апп зөвшөөрсний
            // дараа ЭНЭ хаягаар буцаж манай аппыг идэвхжүүлнэ.
            return try JSONEncoder().encode(["callbackUrl": AppConfig.appToAppCallback])
        case .loginNotify(let register):
            return try JSONEncoder().encode(["register": register])
        case .signPdfStart(let etsi, let digestB64, let fileName):
            // Web demo desktop дээр callbackUrl-ийг хоосон илгээдэг (утас тусдаа
            // төхөөрөмж — Web2App буцалт байхгүй) — яг ижил.
            return try JSONEncoder().encode([
                "etsi": etsi,
                "digestB64": digestB64,
                "fileName": fileName,
                "callbackUrl": "",
            ])
        case .representations(let sid, let token), .children(let sid, let token),
             .certificates(let sid, let token), .dashboard(let sid, let token):
            return try JSONEncoder().encode(["sessionId": sid, "pollToken": token])
        case .esignSign(let personID, let digestB64, let displayText):
            // key=signing — PIN2 (contentCommitment). auth (PIN1) нь ACSP_V2 бүтэц дээр
            // зурдаг тул ДАН-ий payload дээр тохирохгүй; сервер 400 unsupported_key өгнө.
            return try JSONEncoder().encode([
                "personId": personID,
                "digestB64": digestB64,
                "key": "signing",
                "displayText": displayText,
            ])
        default:
            return nil
        }
    }
}

// MARK: - Response Models (web route-уудын яг JSON нэрс)

/// `POST /api/v1/auth/eid/start`, `…/start-id` — платформын өөрийн RP-ийн session.
///
/// Талбарууд snake_case: эдгээр нь Go цөмийн `eid.StartResult`-ынх, харин
/// доорх `/api/*` загварууд Next.js аппынх — хоёр өөр сервер, хоёр өөр хэв маяг.
struct AuthStartResponse: Decodable {
    let sessionID: String
    /// Баталгаажуулах код (5 орон) — утсан дээрхтэй НҮДЭЭР тулгана.
    let verificationCode: String?

    enum CodingKeys: String, CodingKey {
        case sessionID = "session_id"
        case verificationCode = "verification_code"
    }
}

/// `POST /api/v1/auth/eid/poll` → `{state, identity}`.
///
/// `state` нь COMPLETE | RUNNING | EXPIRED | REFUSED. COMPLETE үед сервер нь
/// иргэнийг платформ дээр аль хэдийн бүртгэж, session cookie тавьсан байна —
/// энэ апп cookie хүлээж авдаггүй (`APIClient` дээр зориуд унтраасан) тул
/// зөвхөн `identity`-г л авч Keychain-д snapshot болгоно.
struct AuthPollResponse: Decodable {
    let state: String?
    let identity: AuthIdentity?

    var isComplete: Bool { state == "COMPLETE" }
    var isRunning: Bool { state == "RUNNING" }
}

/// Цөмийн `eid.EIDIdentity` — нэр нь МОНГОЛООР ирдэг тул латин галигийг
/// нөхөх нэмэлт дуудлага (`/api/dashboard`) хэрэггүй.
struct AuthIdentity: Decodable {
    let civilID: String?
    let regNumber: String?
    let firstName: String?
    let lastName: String?
    let familyName: String?
    /// Гэрчилгээний сериал — иргэн ЯМАР гэрчилгээгээр зөвшөөрснийг заана.
    let certificateSerial: String?
    let verifiedStatus: Bool?

    enum CodingKeys: String, CodingKey {
        case civilID = "civil_id"
        case regNumber = "reg_number"
        case firstName = "first_name"
        case lastName = "last_name"
        case familyName = "family_name"
        case certificateSerial = "certificate_serial"
        case verifiedStatus = "verified_status"
    }

    /// «Эцгийн нэр Нэр» (ж: «Цэнддорж Эрдэнэбат»).
    var mongolianName: String? {
        let parts = [lastName, firstName]
            .compactMap { $0?.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
        return parts.isEmpty ? nil : parts.joined(separator: " ")
    }
}

/// `POST /api/start` — QR нэвтрэлтийн session.
struct StartResponse: Decodable {
    let sessionId: String
    /// QR-д кодлох утга (= sessionId).
    let qr: String
    let deviceLinkBase: String?
    /// Баталгаажуулах код — утсан дээрхтэй тулгана.
    let vc: String?
    /// `/api/status` poll-д шаардлагатай token (session эхлүүлэгчид л олгогдоно).
    let pollToken: String?
}

/// `POST /api/login-notify` — РД push session.
struct LoginNotifyResponse: Decodable {
    let sessionId: String
    let vc: String?
    /// `/api/status` poll-д шаардлагатай token (session эхлүүлэгчид л олгогдоно).
    let pollToken: String?
}

/// `GET /api/status?sessionId=` — auth/sign session-ий төлөв.
/// COMPLETE/OK үед web гарын үсгийг баталгаажуулж identity-г задалсан байна.
struct StatusResponse: Decodable {
    let state: String?
    let endResult: String?
    let documentNumber: String?
    let certificateLevel: String?
    /// Cert subject-оос: нэр (given + surname эсвэл CN).
    let name: String?
    /// Cert subject serialNumber → иргэний дугаар.
    let idNumber: String?
    /// Web талын verify алдаа гэх мэт.
    let error: String?
    /// SIGN flow-ийн ТҮҮХИЙ гарын үсэг (ESIGN payload-д хэрэгтэй; PDF stamp хийхгүй).
    let signatureValueB64: String?
    let signatureAlgorithm: String?
    /// Session-ий гэрчилгээ (base64 DER) — нэвтрэлтийн auth cert-ийн эх сурвалж.
    let certificateDerB64: String?

    var isComplete: Bool { state == "COMPLETE" }
    var isOK: Bool { endResult == "OK" }
}

/// `POST /api/sign-pdf-start` — PDF digest-ийг PIN2-оор зурах session.
struct SignPdfStartResponse: Decodable {
    let sessionId: String
    /// Баталгаажуулах код — утсан дээрхтэй тулгана.
    let vc: String?
    /// `/api/status` poll + `/api/sign-pdf-download`-д шаардлагатай token.
    let pollToken: String?
}

/// `POST /api/dashboard` — XYP-ийн хүний хураангуй. Бидэнд хэрэгтэй нь НЭР:
/// `firstName` (нэр), `lastName` (эцэг/эхийн нэр), `familyName` (ургийн овог) —
/// гурвуулаа МОНГОЛООР. Бусад талбарыг (гэрчилгээний тоо, идэвх) энэ апп локалаар
/// угсардаг тул зориуд авахгүй.
struct PersonSummaryResponse: Decodable {
    let firstName: String?
    let lastName: String?
    let familyName: String?

    /// Дэлгэцэнд харагдах монгол нэр: «Эцгийн нэр Нэр» (ж: «Цэнддорж Эрдэнэбат»).
    /// Аль нэг нь дутвал байгаагаараа, хоёулаа хоосон бол `nil` — тэр үед дуудагч
    /// нь гэрчилгээн дэх латин нэр дээрээ үлдэнэ.
    var mongolianName: String? {
        let parts = [lastName, firstName]
            .compactMap { $0?.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
        return parts.isEmpty ? nil : parts.joined(separator: " ")
    }
}

/// `POST /api/representations` — иргэний төлөөлж чадах ACTIVE байгууллагууд
/// (Go `/v3/organization/representations/etsi/{etsi}`).
struct RepresentationsResponse: Decodable {
    let representations: [Representation]
}

struct Representation: Decodable, Identifiable {
    var id: String { orgRegister }
    let orgEtsi: String
    let orgRegister: String
    let orgName: String
    let orgNameEn: String?
    let role: String?
    /// ADMIN — зурагч нэмж/хасна; MANAGER — зөвхөн зурна.
    let rightType: String
    let source: String
}

/// `POST /api/children` — асран хамгаалж буй хүүхдүүд (Go `/v3/person/children/etsi/{etsi}`).
struct ChildrenResponse: Decodable {
    let children: [PersonChild]
}

struct PersonChild: Decodable, Identifiable {
    var id: String { etsi }
    let etsi: String
    let regNo: String
    let name: String
    let birthDate: String?
    /// Идэвхтэй төхөөрөмж/гэрчилгээтэй эсэх — зөвшөөрөл өгсөн ч хүүхэд утсандаа
    /// дуусгаагүй бол false ("Хүлээгдэж буй").
    let registered: Bool
    let certNotAfter: String?
}

/// `POST /api/certificates` — иргэний гэрчилгээ. ESIGN-д `signing` (PIN2) давуу,
/// байхгүй бол `auth` (PIN1). Хоосон мөр нь "хүчинтэй гэрчилгээ олдсонгүй" гэсэн үг.
struct CertificatesResponse: Decodable {
    let signing: String?
    let auth: String?
    let certificateLevel: String?
}

/// `POST /api/esign-sign` — дурын digest-ийг PIN2-оор зурах session.
struct EsignSignResponse: Decodable {
    let sessionId: String
    let vc: String?
    let pollToken: String?
}
