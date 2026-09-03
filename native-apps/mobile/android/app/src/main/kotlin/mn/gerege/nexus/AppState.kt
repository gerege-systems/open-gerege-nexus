package mn.gerege.nexus

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import mn.gerege.nexus.net.ActivityEntry
import mn.gerege.nexus.net.Child
import mn.gerege.nexus.net.Organization
import mn.gerege.nexus.net.StoredIdentity
import mn.gerege.nexus.net.WorkAreaSession
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

/**
 * Аппын төлөв — ширээний `AppState.swift`-ийн Android дүйцэл.
 *
 * Хоёр л дэлгэц: нэвтрэлт эсвэл самбар. Тэр нь Keychain/Keystore дэх snapshot
 * байгаа эсэхээр шийдэгдэнэ — сүлжээ шаардахгүй тул нислэгийн горимд ч апп
 * нээгдэнэ, зөвхөн шинэ өгөгдөл татагдахгүй.
 */
class AppState(application: Application) : AndroidViewModel(application) {
    enum class Screen { LOGIN, DASHBOARD }

    private val store = IdentityStore(application)

    var screen by mutableStateOf(if (store.load() != null) Screen.DASHBOARD else Screen.LOGIN)
        private set
    var identity by mutableStateOf(store.load())
        private set

    val organizations = mutableStateListOf<Organization>()
    val children = mutableStateListOf<Child>()
    var activity by mutableStateOf(store.activity())
        private set

    val fullName: String get() = identity?.fullName.orEmpty()
    val civilId: String get() = identity?.civilId.orEmpty()

    fun didLogin(identity: StoredIdentity) {
        store.save(identity)
        this.identity = identity
        screen = Screen.DASHBOARD
        logActivity("AUTH", "OK")
    }

    fun logout() {
        store.clear()
        // Ажлын мужийн session нь IdentityStore-д биш WebView-ийн cookie санд
        // байдаг тул дээрх цэвэрлэгээнд ОРДОГГҮЙ — тусад нь гаргана.
        WorkAreaSession.clear()
        identity = null
        organizations.clear()
        children.clear()
        activity = emptyList()
        screen = Screen.LOGIN
    }

    /**
     * Байгууллага/хүүхдийн ЗӨВХӨН УНШИХ жагсаалт нь ОДООГООР ХООСОН.
     *
     * Тэдгээр нь eID Mongolia-гийн вэб аппын route-ууд (`/api/representations`,
     * `/api/children`) бөгөөд ТЭР аппын session-ий `pollToken`-оор л уншигдана.
     * Нэвтрэлт платформын өөрийн RP руу шилжсэн тул тэр token байхгүй болов —
     * худал өгөгдөл харуулахын оронд хоосон үлдээнэ.
     *
     * ponytail: платформын esign модульд `GET …/organizations` байгаа; түүнийг
     * холбоход төлөөллийн жагсаалт эргэж ирнэ (platform session cookie хэрэгтэй).
     */

    fun logActivity(type: String, result: String) {
        val entry = ActivityEntry(
            id = System.currentTimeMillis().toString(),
            sessionType = type,
            result = result,
            rpName = AppConfig.BRAND_NAME,
            createdAt = isoNow(),
        )
        store.appendActivity(entry)
        activity = store.activity()
    }

    companion object {
        fun isoNow(): String = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.US)
            .apply { timeZone = TimeZone.getTimeZone("UTC") }.format(Date())

        /** ISO мөрийг «2026.09.02 17:41» болгоно. Задлагдахгүй бол хэвээр нь. */
        fun shortDate(iso: String): String = runCatching {
            val parser = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", Locale.US)
                .apply { timeZone = TimeZone.getTimeZone("UTC") }
            val date = parser.parse(iso.take(19)) ?: return iso
            SimpleDateFormat("yyyy.MM.dd HH:mm", Locale.US).format(date)
        }.getOrDefault(iso)
    }
}
