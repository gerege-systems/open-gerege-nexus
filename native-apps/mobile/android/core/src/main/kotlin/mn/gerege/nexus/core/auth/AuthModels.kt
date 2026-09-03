package mn.gerege.nexus.core.auth

sealed interface LoginPhase {
    data object Idle : LoginPhase
    data object Starting : LoginPhase
    // expiresAtMillis нь UI-гийн countdown-д: сервер хугацаа өгөөгүй бол watch()
    // өөрийн 15 минутын deadline-аа тавьдаг тул энд үргэлж утгатай ирдэг.
    data class Waiting(
        val verificationCode: String,
        val deviceLinkUrl: String?,
        val expiresAtMillis: Long? = null,
    ) : LoginPhase
    data object Success : LoginPhase
    data object Expired : LoginPhase
    data object Refused : LoginPhase
    // message нь хэрэглэгчид харуулах бичвэр; detail нь тэр алдааны техник
    // хэлбэр (exception-ийн бүрэн бичвэр) — зөвхөн «Диагностик» дор гарна.
    data class Error(val message: String, val detail: String? = null) : LoginPhase
}

data class SessionCookie(
    val name: String,
    val value: String,
    val domain: String,
    val path: String,
    val secure: Boolean,
    val httpOnly: Boolean,
)
