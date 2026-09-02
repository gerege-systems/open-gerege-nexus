package mn.gerege.nexus

/**
 * Энэ build-ийн domain шугам.
 *
 * Backend цор ганц. Гэхдээ form factor бүр өөрийн host-оор ханддаг, тэр host нь
 * өөрийн `/api/v1`-ээ мөн үйлчилдэг. Ингэснээр webview доторх дуудлага
 * same-origin болж, session cookie нь `SameSite=Strict` хэвээр ажиллаж, CORS
 * preflight огт үүсэхгүй.
 *
 * Бүртгэл: `native-apps/shared/device_lines.json`.
 */
object DeviceLine {
    /**
     * Энэ build-ийн domain шугам. 2026-08-12-нд асав: DNS (wildcard), nginx
     * vhost, TLS гэрчилгээ, API-гийн origin allowlist дөрвүүлэн бэлэн.
     */
    val origin: String = when (BuildConfig.FORM_FACTOR) {
        "kiosk" -> "https://kiosk.nexus.gerege.mn"
        "pos" -> "https://pos.nexus.gerege.mn"
        else -> "https://mobile.nexus.gerege.mn"
    }

    /**
     * Ажиллахаа больсон хуучин анхдагчууд. Зөвхөн эдгээрийг зөөнө.
     *
     * `https://nexus.gerege.mn` энд ЗОРИУДААР байхгүй: тэр хаяг ижил backend
     * руу очдог тул ажилласаар байгаа бөгөөд түүнийг хүчээр зөөвөл хөтчийн
     * шугамыг санаатай сонгосон суулгацыг булааж авна.
     */
    private val superseded = setOf("http://10.0.2.2:3000", "http://10.0.2.2:8080", "https://android.nexus.gerege.mn")

    fun migrate(stored: String?): String =
        if (stored.isNullOrEmpty() || stored in superseded) origin else stored
}
