package mn.gerege.nexus.net

/** `/api/v1/auth/eid/start`, `…/start-id` — платформын RP-ийн session. */
data class AuthSession(val sessionId: String, val vc: String?)

/** `/api/v1/auth/eid/poll` — терминал төлөв ба иргэний блок. */
data class AuthResult(val state: String?, val identity: AuthIdentity?) {
    val isComplete get() = state == "COMPLETE"
}

/**
 * Цөмийн `eid.EIDIdentity`. Нэр нь МОНГОЛООР ирдэг тул латин галигийг нөхөх
 * нэмэлт дуудлага (`/api/dashboard`) хэрэггүй.
 */
data class AuthIdentity(
    val civilId: String?,
    val regNumber: String?,
    val firstName: String?,
    val lastName: String?,
    val certificateSerial: String?,
    val verified: Boolean,
) {
    /** «Эцгийн нэр Нэр» (ж: «Цэнддорж Эрдэнэбат»). */
    val mongolianName: String?
        get() = listOfNotNull(lastName, firstName)
            .map { it.trim() }.filter { it.isNotEmpty() }
            .takeIf { it.isNotEmpty() }?.joinToString(" ")
}

/** `/api/start`, `/api/login-notify` — session эхлэх хариу. */
data class StartSession(val sessionId: String, val vc: String?, val pollToken: String)

/** `/api/status` — session-ий төлөв (ширээний `StatusResponse`-той ижил талбарууд). */
data class AuthStatus(
    val state: String?,
    val endResult: String?,
    val documentNumber: String?,
    val certificateLevel: String?,
    val name: String?,
    val idNumber: String?,
    val error: String?,
) {
    val isComplete get() = state == "COMPLETE"
    val isOk get() = endResult == "OK"
}

/**
 * `/api/dashboard` — XYP-ийн хүний хураангуй.
 *
 * Гэрчилгээний subject дэх нэр нь ЛАТИН галиг тул дэлгэцэнд харагдах нэрийг
 * эндээс авна — ширээ, iOS хоёрын дүрэмтэй ижил.
 */
data class PersonSummary(val firstName: String?, val lastName: String?, val familyName: String?) {
    val mongolianName: String?
        get() = listOfNotNull(lastName, firstName)
            .map { it.trim() }.filter { it.isNotEmpty() }
            .takeIf { it.isNotEmpty() }?.joinToString(" ")
}

data class Organization(val orgRegister: String, val orgName: String, val rightType: String)
data class Child(val regNo: String, val name: String, val registered: Boolean)

/** Нэвтэрсэн иргэний snapshot — Keystore-оор шифрлэгдэж хадгалагдана. */
data class StoredIdentity(
    val documentNumber: String,
    val fullName: String,
    val civilId: String,
    val nationalId: String,
    val certificateLevel: String,
    val loginAt: String,
)

/** Энэ ТӨХӨӨРӨМЖ дээрх үйлдлийн локал бүртгэл (v3-д серверийн түүх байхгүй). */
data class ActivityEntry(
    val id: String,
    val sessionType: String,
    val result: String,
    val rpName: String,
    val createdAt: String,
)
