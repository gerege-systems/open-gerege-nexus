package mn.gerege.nexus.net

import android.webkit.CookieManager
import mn.gerege.nexus.AppConfig

/**
 * Native нэвтрэлтийг ажлын муж руу дамжуулагч — iOS-ийн `WorkAreaSession.swift`-ийн дүйцэл.
 *
 * OkHttp нь анхдагчаараа cookie хадгалдаггүй (`CookieJar.NO_COOKIES`) бөгөөд
 * тэр нь зөв хэвээр: native дуудлагууд session биш, `sessionId` + `pollToken`
 * хосоор ажилладаг. Гэвч платформын нэвтрэлт (`/api/v1/auth/eid/poll`)
 * амжилттай болоход сервер `session_token`-ыг `Set-Cookie`-гээр буцаадаг ба
 * тэр нь **ажлын мужийн цорын ганц нэвтрэлт**: төхөөрөмжийн шугам дээр
 * web-ийн `/login` хаалттай (`frontend/proxy.ts`) тул WebView өөрөө нэвтэрч
 * ЧАДАХГҮЙ.
 *
 * Толгойг задлаад дахин угсрахгүй, `CookieManager` руу түүхийг нь ХЭВЭЭР
 * дамжуулна: `HttpOnly`, `Secure`, `SameSite`, дуусах хугацаа бүгд үлдэнэ.
 */
object WorkAreaSession {

    /** `backend/internal/kernel/security/csrf.go` → `TenantSessionCookie`. */
    private const val COOKIE_NAME = "session_token"

    /** `Set-Cookie` толгой бүрд нэг удаа. Бусад cookie-г алгасна. */
    fun capture(setCookieHeader: String) {
        if (!setCookieHeader.startsWith("$COOKIE_NAME=")) return
        CookieManager.getInstance().apply {
            setAcceptCookie(true)
            setCookie(AppConfig.baseUrl, setCookieHeader)
            flush()
        }
    }

    /**
     * Гарахад ажлын мужийг мөн гаргана — эс бөгөөс дараагийн хүн өмнөх хүний
     * мужийг табаа дарахад л харна.
     *
     * ponytail: тухайн нэг cookie-г нэрээр нь устгах API байхгүй тул хугацааг
     * нь өнгөрүүлж дарна — бүх cookie-г цэвэрлэвэл хүний өөрийн хэл, харагдац
     * зэрэг ажлын мужийн бусад тохиргоо хамт алга болно.
     */
    fun clear() {
        CookieManager.getInstance().apply {
            setCookie(AppConfig.baseUrl, "$COOKIE_NAME=; Path=/; Max-Age=0")
            flush()
        }
    }
}
