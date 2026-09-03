import Foundation
import WebKit

/// Native нэвтрэлтийг ажлын муж руу дамжуулагч.
///
/// `APIClient` нь cookie-г ЗОРИУДААР хүлээж авдаггүй (`httpCookieAcceptPolicy
/// = .never`) бөгөөд тэр нь зөв хэвээр: native дуудлагууд session биш,
/// `sessionId` + `pollToken` хосоор ажилладаг тул ambient authority шаардахгүй.
///
/// Гэвч платформын нэвтрэлт (`/api/v1/auth/eid/poll`) амжилттай болоход сервер
/// `session_token`-ыг `Set-Cookie`-гээр буцаадаг ба тэр нь **ажлын мужийн
/// цорын ганц нэвтрэлт**: төхөөрөмжийн шугам дээр web-ийн `/login` хаалттай
/// (`frontend/proxy.ts` — тэнд нэвтрэлт бол native UI) тул webview өөрөө
/// нэвтэрч ЧАДАХГҮЙ. Энэ дамжуулалтгүйгээр ажлын муж нь native талд нэвтэрсэн
/// хүнийг ч «нэвтрээгүй» гэж уншина — яг тэр байдал 2026-09-03 хүртэл
/// үргэлжилсэн.
///
/// Cookie-г задлаад дахин угсрахгүй, серверийн толгойг `HTTPCookie`-оор
/// шууд уншина: `HttpOnly`, `Secure`, `SameSite`, дуусах хугацаа бүгд хэвээр
/// үлдэнэ. Гараар барьсан cookie нь тэдгээрийг чимээгүй унагаадаг.
enum WorkAreaSession {

    /// `backend/internal/kernel/security/csrf.go` → `TenantSessionCookie`.
    static let cookieName = "session_token"

    /// Хариунд session cookie байвал webview-ийн сан руу суулгана.
    ///
    /// Бусад бүх cookie-г алгасна: энэ нь cookie jar биш, нэг тодорхой
    /// нэвтрэлтийн гүүр.
    static func capture(from response: HTTPURLResponse, for url: URL?) {
        guard let url,
              let fields = response.allHeaderFields as? [String: String] else { return }
        let session = HTTPCookie.cookies(withResponseHeaderFields: fields, for: url)
            .filter { $0.name == cookieName && !$0.value.isEmpty }
        guard !session.isEmpty else { return }
        Task { @MainActor in
            for cookie in session {
                await WKWebsiteDataStore.default().httpCookieStore.setCookie(cookie)
            }
        }
    }

    /// Гарахад ажлын мужийг мөн гаргана. Native тал гарчихаад webview нь
    /// нэвтэрсэн хэвээр үлдэх нь дараагийн хүнд өмнөх хүний мужийг үзүүлнэ.
    static func clear() {
        Task { @MainActor in
            let store = WKWebsiteDataStore.default().httpCookieStore
            for cookie in await store.allCookies() where cookie.name == cookieName {
                await store.deleteCookie(cookie)
            }
        }
    }
}
