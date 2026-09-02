package mn.gerege.nexus.core.auth

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.HttpUrl.Companion.toHttpUrl
import java.util.concurrent.ConcurrentHashMap

data class EIDStart(val sessionId: String, val verificationCode: String, val deviceLinkUrl: String?, val expiresAt: String?)

class AuthApi(
    private val apiBase: String = "http://10.0.2.2:8080/api/v1/",
    private val jar: MemoryCookieJar = MemoryCookieJar(),
    private val client: OkHttpClient = OkHttpClient.Builder().cookieJar(jar).build(),
) {
    private val json = Json { ignoreUnknownKeys = true }

    suspend fun password(email: String, password: String) {
        post("auth/login", """{"email":${quote(email)},"password":${quote(password)}}""")
    }

    suspend fun staffPin(pin: String, deviceToken: String) {
        post("devices/staff/pin", """{"pin":${quote(pin)}}""", deviceToken)
    }
    suspend fun registerPushToken(token:String,appId:String){post("push-tokens","""{"provider":"FCM","token":${quote(token)},"app_id":${quote(appId)}}""")}

    suspend fun startPush(nationalId: String, callbackUrl: String): EIDStart {
        val value = post("auth/eid/start-id", """{"national_id":${quote(nationalId.trim().uppercase())},"callbackUrl":${quote(callbackUrl)}}""").jsonObject
        return EIDStart(
            value.getValue("session_id").jsonPrimitive.content,
            value.getValue("verification_code").jsonPrimitive.content,
            value["device_link_url"]?.jsonPrimitive?.contentOrNull,
            value["expires_at"]?.jsonPrimitive?.contentOrNull,
        )
    }

    suspend fun startDeviceLink(callbackUrl: String): EIDStart {
        val value = post("auth/eid/start", """{"callbackUrl":${quote(callbackUrl)}}""").jsonObject
        return EIDStart(
            value.getValue("session_id").jsonPrimitive.content,
            value.getValue("verification_code").jsonPrimitive.content,
            value["device_link_url"]?.jsonPrimitive?.contentOrNull,
            value["expires_at"]?.jsonPrimitive?.contentOrNull,
        )
    }

    suspend fun poll(sessionId: String): String = post("auth/eid/poll", """{"session_id":${quote(sessionId)}}""")
        .jsonObject.getValue("state").jsonPrimitive.content.uppercase()

    fun sessionCookies(): List<SessionCookie> = jar.all().filter { it.name == "session_token" }.map {
        SessionCookie(it.name, it.value, it.domain, it.path, it.secure, it.httpOnly)
    }

    private suspend fun post(path: String, body: String, deviceToken: String? = null) = withContext(Dispatchers.IO) {
        val builder = Request.Builder().url(apiBase + path)
            .header("Accept-Language", "mn")
            .header("Origin", apiBase.toHttpUrl().run { "$scheme://$host${if (port == 80 || port == 443) "" else ":$port"}" })
        if (deviceToken != null) builder.header("Authorization", "Device $deviceToken")
        val request = builder.post(body.toRequestBody("application/json".toMediaType())).build()
        client.newCall(request).execute().use { response ->
            val text = response.body?.string().orEmpty()
            if (!response.isSuccessful) {
                val message = runCatching { json.parseToJsonElement(text).jsonObject["error"]?.jsonPrimitive?.content }.getOrNull()
                error(message ?: "HTTP ${response.code}")
            }
            json.parseToJsonElement(text.ifBlank { "{}" })
        }
    }

    private fun quote(value: String) = JsonPrimitive(value).toString()
}

class MemoryCookieJar : CookieJar {
    private val cookies = ConcurrentHashMap<String, Cookie>()
    override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
        cookies.forEach { this.cookies["${it.domain}|${it.path}|${it.name}"] = it }
    }
    override fun loadForRequest(url: HttpUrl): List<Cookie> = all().filter { it.matches(url) }
    fun all(): List<Cookie> {
        val now = System.currentTimeMillis()
        cookies.entries.removeIf { it.value.expiresAt < now }
        return cookies.values.toList()
    }
}
