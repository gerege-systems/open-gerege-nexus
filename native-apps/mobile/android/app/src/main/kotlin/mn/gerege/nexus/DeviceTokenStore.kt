package mn.gerege.nexus

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

class DeviceTokenStore(context: Context) {
    private val alias = "mn.gerege.nexus.device-token"
    private val preferences = context.getSharedPreferences("native-secure-envelope", Context.MODE_PRIVATE)
    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(alias, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build())
        return generator.generateKey()
    }
    fun save(token: String) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding"); cipher.init(Cipher.ENCRYPT_MODE, key())
        preferences.edit().putString("token", Base64.encodeToString(cipher.iv + cipher.doFinal(token.toByteArray()), Base64.NO_WRAP)).apply()
    }
    fun load(): String? = runCatching {
        val envelope = Base64.decode(preferences.getString("token", null) ?: return null, Base64.NO_WRAP)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, envelope.copyOfRange(0, 12)))
        String(cipher.doFinal(envelope.copyOfRange(12, envelope.size)))
    }.getOrNull()
    fun clear() { preferences.edit().remove("token").apply(); KeyStore.getInstance("AndroidKeyStore").apply { load(null); deleteEntry(alias) } }
}
