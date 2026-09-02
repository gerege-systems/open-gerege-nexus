package mn.gerege.nexus.core.auth

sealed interface LoginPhase {
    data object Idle : LoginPhase
    data object Starting : LoginPhase
    data class Waiting(val verificationCode: String, val deviceLinkUrl: String?) : LoginPhase
    data object Success : LoginPhase
    data object Expired : LoginPhase
    data object Refused : LoginPhase
    data class Error(val message: String) : LoginPhase
}

data class SessionCookie(
    val name: String,
    val value: String,
    val domain: String,
    val path: String,
    val secure: Boolean,
    val httpOnly: Boolean,
)
