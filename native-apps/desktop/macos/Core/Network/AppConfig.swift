import Foundation

/// Холболтын тохиргоо — **first-party** desktop app.
///
/// iOS app-тай ижил зарчмаар энэ app RP биш: RP secret, RP UUID зэрэг
/// юу ч агуулахгүй. Бүх дуудлага өөрийн web backend-ийн нийтийн
/// `/api/*` route-уудаар дамжина (browser-тэй яг ижил зам) — Go RP-API-ийн
/// secret зөвхөн web серверийн орчинд үлдэнэ.
///
/// Эх сурвалжийн дараалал (эхний хоосон биш нь ялна):
///   UserDefaults override (Settings UI) → environment variable → default.
enum AppConfig {

    /// UserDefaults key (Settings UI-тай хуваалцана).
    static let baseURLKey = "API_BASE_URL_OVERRIDE"

    /// Энэ байрлуулалт өөрийгөө юу гэж нэрлэх вэ.
    ///
    /// Кодод бичсэн нэр нь тэр нэрийг образ дагуулна — платформын
    /// `BRAND_NAME`-тэй яг ижил шалтгаанаар энд тогтмол биш, орчноос
    /// уншигдана. Тавигдаагүй үеийн утга нь энэ байрлуулалтынх.
    static var brandName: String {
        let env = ProcessInfo.processInfo.environment["BRAND_NAME"] ?? ""
        let trimmed = env.trimmingCharacters(in: .whitespaces)
        return trimmed.isEmpty ? "Gerege Nexus" : trimmed
    }

    /// Локал лог/самбарт харагдах шошго.
    ///
    /// Энэ нь RP-ийн нэр БИШ: session-ууд платформын өөрийн ганц RP-ээр үүсдэг
    /// (`rpclient.ts § RP_SELF`) бөгөөд иргэн утсан дээрээ ТЭР нэрийг харна.
    /// Энд байгаа нь зөвхөн энэ төхөөрөмжийн локал бүртгэлийн шошго тул
    /// байрлуулалтын брэндийг дагана.
    static var serviceName: String { brandName }

    /// App2App-ийн БУЦАХ хаяг — session эхлүүлэхэд `callbackUrl`-аар илгээнэ.
    ///
    /// Утсан дээр зөвшөөрөгч нь ӨӨРӨӨ тэр төхөөрөмж тул eID апп зөвшөөрөл авмагц
    /// энэ схемээр манай аппыг эргүүлж идэвхжүүлнэ (`?sessionId=…` залгаж өгнө;
    /// session-ий үр дүнг poll аль хэдийн барьж байгаа).
    ///
    /// Ширээн дээр ХООСОН байх нь санаатай: тэнд зөвшөөрөгч нь ӨӨР төхөөрөмж —
    /// буцах хаяг өгвөл eID апп ТЭР УТСАН дээр байхгүй аппыг нээхийг оролдоно.
    ///
    /// Nexus backend бүтэн URI-г `EID_APP_CALLBACKS`-аар, eID сервер
    /// түүний scheme-ийг RP-ийн `callback_hosts`-оор шалгана
    /// (`NormalizeCallback`). Аль нэгэнд бүртгэгдээгүй бол ЧИМЭЭГҮЙ хаягдана — app2app
    /// буцалт ажиллахгүй байвал хамгийн түрүүнд тэнд харна.
    static let appToAppCallback: String = {
        #if os(iOS)
        return "gerege-nexus://auth"
        #else
        return ""
        #endif
    }()

    /// Web app-ын суурь URL (`/api/*` route-ууд энд байрлана).
    static var baseURL: String {
        if let o = UserDefaults.standard.string(forKey: baseURLKey),
           !o.trimmingCharacters(in: .whitespaces).isEmpty {
            return normalize(o)
        }
        if let e = ProcessInfo.processInfo.environment["API_BASE_URL"], !e.isEmpty {
            return normalize(e)
        }
        // DEBUG-д ч ижил анхдагч. Өмнө нь DEBUG нь `http://localhost:3000`-д ордог байсан тул
        // локал web сервер ажиллуулаагүй хүн (тестлэгч, дизайнер, эсвэл зүгээр л Debug build
        // ажиллуулсан) "Could not connect to the server" аваад гацдаг байв. Локал backend руу
        // заахдаа Settings → Сервер, эсвэл `API_BASE_URL=http://localhost:3000` env.
        // ЭНЭ БАЙРЛУУЛАЛТЫН төхөөрөмжийн шугам. Апп нь eID-ийн `/api/*`
        // route-уудыг ШУУД биш, өөрийн хостоор дамжуулан дуудна: nginx тэр
        // замуудыг eID платформ руу proxy хийнэ
        // (nginx/device-lines.nexus.gerege.mn.conf).
        //
        // Ингэсний шалтгаан нь гоо сайхан биш: клиент нэг гарал мэддэг бол
        // webview доторх ажлын муж, native дуудлага хоёр НЭГ origin дээр
        // үлдэж, session cookie нь SameSite=Strict хэвээр байна.
        //
        // TLS pinning нь Let's Encrypt-ийн E7/E8 ЗАВСРЫН гэрчилгээг заадаг
        // тул (leaf биш) энэ хост certbot-оор гэрчилгээ авдаг л бол pin
        // хэвээр таарна.
        // ЭНЭ БАЙРЛУУЛАЛТЫН ширээний шугам.
        //
        // Апп нь eID-ийн `/api/*` route-уудыг ШУУД биш, өөрийн хостоор
        // дамжуулан дуудна: nginx тэдгээрийг eID платформ руу proxy хийнэ
        // (nginx/device-lines.nexus.gerege.mn.conf, snippets/eid-api-proxy.conf).
        //
        // Ингэсний шалтгаан нь гоо сайхан биш: клиент нэг гарал мэддэг бол
        // webview доторх ажлын муж, native дуудлага хоёр НЭГ origin дээр
        // үлдэж, session cookie нь SameSite=Strict хэвээр байна.
        //
        // Энэ хостыг заахын өмнө DNS → nginx → certbot гурав бэлэн байх
        // ЁСТОЙ. Тэр дараалал алдагдвал апп «A TLS error caused the secure
        // connection to fail» дээр буудаг — нэг удаа яг тэр болсон.
        //
        // TLS pinning нь гинжний ISRG-ийн үндсийг заадаг (Let's Encrypt
        // завсрынхаа эргүүлдэг тул), энэ хост certbot-оор гэрчилгээ авдаг л
        // бол таарна.
        // Шугам нь ПЛАТФОРМ биш FORM FACTOR-ыг нэрлэнэ: ширээн дээрх Mac ба
        // Windows нэг шугам, гарын дээрх iOS ба Android нөгөө шугам
        // (native-apps/shared/device_lines.json).
        #if os(iOS)
        return "https://mobile.nexus.gerege.mn"
        #else
        return "https://desktop.nexus.gerege.mn"
        #endif
    }

    private static func normalize(_ url: String) -> String {
        var s = url.trimmingCharacters(in: .whitespaces)
        while s.hasSuffix("/") { s.removeLast() }
        return s
    }
}
