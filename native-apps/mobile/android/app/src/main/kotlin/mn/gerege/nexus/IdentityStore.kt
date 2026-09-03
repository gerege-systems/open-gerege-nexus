package mn.gerege.nexus

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import mn.gerege.nexus.net.ActivityEntry
import mn.gerege.nexus.net.StoredIdentity
import org.json.JSONArray
import org.json.JSONObject
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Нэвтэрсэн иргэний snapshot — iOS-ийн Keychain-ий дүйцэл.
 *
 * Bearer session БАЙХГҮЙ (ширээ, iOS-тэй ижил загвар): `documentNumber` нь
 * дараагийн үйлдлийн бариул. Тиймээс энэ файл алдагдвал хэн ч нэвтэрч чадахгүй
 * — Android Keystore-ийн түлхүүрээр (AES-GCM) шифрлэж хадгална. Түлхүүр нь
 * төхөөрөмжөөс гардаггүй тул нөөцлөлт хуулсан ч задрахгүй.
 */
class IdentityStore(context: Context) {
    private val alias = "mn.gerege.nexus.identity"
    private val prefs = context.applicationContext.getSharedPreferences("eid.identity", Context.MODE_PRIVATE)

    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(alias, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(
            KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build()
        )
        return generator.generateKey()
    }

    fun save(identity: StoredIdentity) {
        val payload = JSONObject()
            .put("documentNumber", identity.documentNumber)
            .put("fullName", identity.fullName)
            .put("civilId", identity.civilId)
            .put("nationalId", identity.nationalId)
            .put("certificateLevel", identity.certificateLevel)
            .put("loginAt", identity.loginAt)
            .toString()
        val cipher = Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, key()) }
        val envelope = cipher.iv + cipher.doFinal(payload.toByteArray())
        prefs.edit().putString("identity", Base64.encodeToString(envelope, Base64.NO_WRAP)).apply()
    }

    fun load(): StoredIdentity? = runCatching {
        val raw = prefs.getString("identity", null) ?: return null
        val envelope = Base64.decode(raw, Base64.NO_WRAP)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, envelope.copyOfRange(0, 12)))
        val json = JSONObject(String(cipher.doFinal(envelope.copyOfRange(12, envelope.size))))
        StoredIdentity(
            json.optString("documentNumber"), json.optString("fullName"), json.optString("civilId"),
            json.optString("nationalId"), json.optString("certificateLevel"), json.optString("loginAt"),
        )
    }.getOrNull()

    fun clear() {
        prefs.edit().clear().apply()
        runCatching { KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.deleteEntry(alias) }
    }

    // ── Локал үйл ажиллагааны лог ──────────────────────────────────────────
    // Серверт иргэний бүх session-ий түүх БАЙХГҮЙ тул энэ жагсаалт нь ЭНЭ
    // төхөөрөмжийн түүх. Утас, мак хоёр өөр өөр жагсаалт харуулах нь хэвийн.

    fun activity(): List<ActivityEntry> = runCatching {
        val array = JSONArray(prefs.getString("activity", "[]"))
        (0 until array.length()).map { index ->
            val item = array.getJSONObject(index)
            ActivityEntry(item.optString("id"), item.optString("sessionType"),
                item.optString("result"), item.optString("rpName"), item.optString("createdAt"))
        }
    }.getOrDefault(emptyList())

    fun appendActivity(entry: ActivityEntry) {
        val kept = (listOf(entry) + activity()).take(100)
        val array = JSONArray()
        kept.forEach {
            array.put(JSONObject()
                .put("id", it.id).put("sessionType", it.sessionType).put("result", it.result)
                .put("rpName", it.rpName).put("createdAt", it.createdAt))
        }
        prefs.edit().putString("activity", array.toString()).apply()
    }
}
