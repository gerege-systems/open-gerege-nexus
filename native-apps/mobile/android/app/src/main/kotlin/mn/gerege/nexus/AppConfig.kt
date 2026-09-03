package mn.gerege.nexus

import android.content.Context
import android.content.SharedPreferences

/**
 * Ширээний `Core/Network/AppConfig.swift`-ийн Android дүйцэл.
 *
 * Зарчим нь ижил: клиентэд RP secret БАЙХГҮЙ. Бүх дуудлага өөрийн web
 * backend-ийн нийтийн `/api/…` route-уудаар явна (хөтөчтэй яг ижил зам), RP-ийн
 * нууцыг зөвхөн web сервер барина.
 */
object AppConfig {
    const val BRAND_NAME = "Gerege Nexus"
    private const val PREFS = "eid.settings"
    const val BASE_URL_KEY = "API_BASE_URL_OVERRIDE"

    /**
     * App2App буцах хаяг. Manifest-ийн intent-filter-тэй (`gerege-nexus://auth`)
     * ЯГ таарах ёстой. Nexus backend-ийн `EID_APP_CALLBACKS`-д
     * энэ бүтэн URI, eID RP-ийн `callback_hosts`-д `gerege-nexus://`
     * scheme бүртгэгдсэн байх ёстой. Аль нэг нь дутвал буцалт ажиллахгүй.
     */
    const val APP_TO_APP_CALLBACK = "gerege-nexus://auth"

    /** Гарын шугам — iOS-тэй ИЖИЛ хост (`shared/device_lines.json` → mobile). */
    const val DEFAULT_BASE_URL = "https://mobile.nexus.gerege.mn"

    private lateinit var prefs: SharedPreferences

    fun init(context: Context) {
        prefs = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    }

    var baseUrl: String
        get() = prefs.getString(BASE_URL_KEY, null)?.trim()?.trimEnd('/')
            ?.takeIf { it.isNotEmpty() } ?: DEFAULT_BASE_URL
        set(value) {
            val normalized = value.trim().trimEnd('/')
            prefs.edit().apply {
                if (normalized.isEmpty()) remove(BASE_URL_KEY) else putString(BASE_URL_KEY, normalized)
            }.apply()
        }

    val host: String get() = baseUrl.removePrefix("https://").removePrefix("http://").substringBefore('/')

    fun prefs(): SharedPreferences = prefs
}
