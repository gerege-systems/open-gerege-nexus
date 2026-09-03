package mn.gerege.nexus

import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.File

/**
 * Ажлын мужийн нэвтрэлт дөрвөн файлын НЭГ мөрөөс хамаарна: серверийн
 * cookie-гийн нэр.
 *
 * Түүнийг Go тал дээр сольвол хаана ч компайлын алдаа гарахгүй — Swift ба
 * Kotlin тал хуучин нэрээ хайсаар байж, гурван клиентийн ажлын муж чимээгүйхэн
 * «нэвтрээгүй» болно. Native тал нэвтэрсэн хэвээр байх тул хүн юу ч буруу
 * болсныг мэдэхгүй: зүгээр л таб нь хоосон.
 *
 * Тиймээс гурвуулыг эх кодоос нь уншиж тулгана. Гүйцэтгэлийн тест биш,
 * гэрээний тест — `CallbackContractTest`-тэй ижил зорилготой.
 */
class WorkAreaSessionContractTest {

    private val repo = File("../../../..")

    private fun read(path: String) = File(repo, path).readText()

    @Test
    fun everyClientNamesTheServersSessionCookie() {
        // backend/internal/kernel/security/csrf.go:
        //     const TenantSessionCookie = "session_token"
        val server = Regex("""TenantSessionCookie\s*=\s*"([^"]+)"""")
            .find(read("backend/internal/kernel/security/csrf.go"))
            ?.groupValues?.get(1)
        assertEquals("csrf.go-оос TenantSessionCookie олдсонгүй", true, server != null)

        val swift = Regex("""cookieName\s*=\s*"([^"]+)"""")
            .find(read("native-apps/desktop/macos/Core/Network/WorkAreaSession.swift"))
            ?.groupValues?.get(1)
        val kotlin = Regex("""COOKIE_NAME\s*=\s*"([^"]+)"""")
            .find(read("native-apps/mobile/android/app/src/main/kotlin/mn/gerege/nexus/net/WorkAreaSession.kt"))
            ?.groupValues?.get(1)

        assertEquals("macOS/iOS-ийн cookie нэр серверийнхтэй зөрж байна", server, swift)
        assertEquals("Android-ийн cookie нэр серверийнхтэй зөрж байна", server, kotlin)
    }

    @Test
    fun theWorkAreaLoadsTheSameOriginTheApiCalls() {
        // Cookie нь host-only. WebView өөр хостоос ачаалагдвал session хэзээ ч
        // илгээгдэхгүй бөгөөд алдаа нь 401 биш — зүгээр л «нэвтрээгүй» дэлгэц.
        val screen = read("native-apps/mobile/android/app/src/main/kotlin/mn/gerege/nexus/ui/PlatformScreen.kt")
        assertEquals(
            "Ажлын муж AppConfig.baseUrl-ээс ачаалагдах ЁСТОЙ — өөр хаяг session-ыг тасална",
            true,
            screen.contains("loadUrl(AppConfig.baseUrl)"),
        )
    }
}
