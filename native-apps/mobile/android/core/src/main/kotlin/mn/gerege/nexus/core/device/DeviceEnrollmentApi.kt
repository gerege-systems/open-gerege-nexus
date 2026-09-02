package mn.gerege.nexus.core.device

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

data class EnrolledDevice(val id: String, val tenantId: String, val token: String)
data class DeviceIdentity(val id: String, val tenantId: String, val name: String, val platform: String, val formFactor: String, val status: String)

class DeviceEnrollmentApi(
    private val apiBase: String = "http://10.0.2.2:8080/api/v1/",
    private val client: OkHttpClient = OkHttpClient(),
) {
    private val json = Json { ignoreUnknownKeys = true }

    suspend fun enroll(code: String, name: String, platform: String, formFactor: String, site: String, appVersion: String, osVersion: String): EnrolledDevice {
        val body = mapOf("code" to code, "name" to name, "platform" to platform, "form_factor" to formFactor,
            "site" to site, "app_version" to appVersion, "os_version" to osVersion).entries.joinToString(",", "{", "}") { (key, value) -> "${quote(key)}:${quote(value)}" }
        val value = request("devices/enroll", body).jsonObject
        return EnrolledDevice(value.getValue("device_id").jsonPrimitive.content, value.getValue("tenant_id").jsonPrimitive.content, value.getValue("device_token").jsonPrimitive.content)
    }

    suspend fun me(token: String): DeviceIdentity {
        val value = request("devices/me", null, token).jsonObject
        return DeviceIdentity(value.getValue("id").jsonPrimitive.content, value.getValue("tenant_id").jsonPrimitive.content,
            value.getValue("name").jsonPrimitive.content, value.getValue("platform").jsonPrimitive.content,
            value.getValue("form_factor").jsonPrimitive.content, value.getValue("status").jsonPrimitive.content)
    }
    suspend fun telemetry(token:String,event:String,level:String="INFO"){request("devices/telemetry","""{"events":[{"level":${quote(level)},"event":${quote(event)},"payload":{"form_factor":${quote("android")}},"occurred_at":${quote(java.time.Instant.now().toString())}}]}""",token)}
    suspend fun rotateToken(token:String):String=request("devices/token/rotate","{}",token).jsonObject.getValue("device_token").jsonPrimitive.content

    private suspend fun request(path: String, body: String?, token: String? = null) = withContext(Dispatchers.IO) {
        val builder = Request.Builder().url(apiBase + path).header("Accept-Language", "mn")
        if (token != null) builder.header("Authorization", "Device $token")
        if (body == null) builder.get() else builder.post(body.toRequestBody("application/json".toMediaType()))
        client.newCall(builder.build()).execute().use { response ->
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
