package mn.gerege.nexus.core.auth

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.time.Instant
import java.util.concurrent.atomic.AtomicLong
import javax.net.ssl.SSLException

class AuthStateMachine(private val api: AuthApi, private val scope: CoroutineScope) {
    private val ticket = AtomicLong()
    private var attempt: Job? = null
    private val mutablePhase = MutableStateFlow<LoginPhase>(LoginPhase.Idle)
    val phase: StateFlow<LoginPhase> = mutablePhase.asStateFlow()

    fun cancel() {
        ticket.incrementAndGet(); attempt?.cancel(); attempt = null
        mutablePhase.value = LoginPhase.Idle
    }

    fun password(email: String, password: String) = begin { mine ->
        api.password(email, password)
        if (ticket.get() == mine) mutablePhase.value = LoginPhase.Success
    }

    fun staffPin(pin: String, deviceToken: String) = begin { mine ->
        api.staffPin(pin, deviceToken)
        if (ticket.get() == mine) mutablePhase.value = LoginPhase.Success
    }

    fun push(nationalId: String, callbackUrl: String = "") = begin { mine ->
        watch(api.startPush(nationalId, callbackUrl), mine)
    }

    fun appToApp(callbackUrl: String) = begin { mine ->
        watch(api.startDeviceLink(callbackUrl), mine)
    }

    fun sessionCookies() = api.sessionCookies()
    fun registerPushToken(token:String,appId:String)=scope.launch { runCatching { api.registerPushToken(token,appId) } }

    private fun begin(block: suspend (Long) -> Unit) {
        cancel()
        val mine = ticket.incrementAndGet()
        mutablePhase.value = LoginPhase.Starting
        attempt = scope.launch {
            try { block(mine) }
            catch (_: CancellationException) { }
            catch (error: Throwable) {
                if (ticket.get() == mine) mutablePhase.value = describeFailure(error)
            }
        }
    }

    private suspend fun watch(start: EIDStart, mine: Long) {
        val deadline = start.expiresAt?.let { runCatching { Instant.parse(it).toEpochMilli() }.getOrNull() }
            ?: System.currentTimeMillis() + 15 * 60_000
        mutablePhase.value = LoginPhase.Waiting(start.verificationCode, start.deviceLinkUrl, deadline)
        var failures = 0
        while (ticket.get() == mine) {
            if (System.currentTimeMillis() >= deadline) { mutablePhase.value = LoginPhase.Expired; return }
            try {
                when (api.poll(start.sessionId)) {
                    "COMPLETE" -> { mutablePhase.value = LoginPhase.Success; return }
                    "EXPIRED" -> { mutablePhase.value = LoginPhase.Expired; return }
                    "REFUSED" -> { mutablePhase.value = LoginPhase.Refused; return }
                }
                failures = 0
            } catch (cancelled: CancellationException) { throw cancelled }
            catch (error: Throwable) { if (++failures > 3) throw error }
            delay(400)
        }
    }
}

/**
 * Алдааг хэрэглэгчийн хэлээр.
 *
 * API-гийн өөрийн хариу (HTTP 4xx/5xx-ийн `error` талбар) `AuthApi.post`-оос
 * IllegalStateException болж ирдэг бөгөөд сервер түүнийг аль хэдийн монголоор
 * бичсэн байдаг — тэр мессежийг хэвээр дамжуулна. Харин сүлжээний давхаргын
 * алдаа (DNS олдохгүй, TLS тогтохгүй, хугацаа хэтрэх) Java-гийн англи бичвэртэй
 * ирдэг: «Unable to resolve host "…": No address associated with hostname».
 * Түүнийг нэвтрэх дэлгэц дээр шууд асгах нь оношлогооны логийг хэрэглэгчид
 * уншуулсантай адил. Тиймээс энд хүний хэлээр орчуулж, техник бичвэрийг нь
 * `detail`-д хадгална — «Диагностик» товч түүнийг л дэлгэнэ.
 *
 * Дараалал чухал: SSLException, UnknownHostException, SocketTimeoutException,
 * ConnectException бүгд IOException-ийн удам тул ерөнхий салбар нь хамгийн
 * сүүлд.
 */
internal fun describeFailure(error: Throwable): LoginPhase.Error {
    val message = when (error) {
        is UnknownHostException -> "Сервертэй холбогдож чадсангүй — интернэт холболтоо шалгана уу"
        is SSLException -> "Серверийн аюулгүй холболт (TLS) тогтоогдсонгүй"
        is SocketTimeoutException -> "Сервер хариу өгсөнгүй — дахин оролдоно уу"
        is ConnectException -> "Сервертэй холбогдож чадсангүй"
        is IOException -> "Сүлжээний алдаа — дахин оролдоно уу"
        else -> error.message ?: "Нэвтрэх хүсэлт амжилтгүй"
    }
    return LoginPhase.Error(message, error.toString())
}
