package mn.gerege.nexus

import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * App2App-ийн буцах хаяг дөрвөн гэрээнд зэрэг бүртгэгдсэн байх ёстой:
 *
 *   1. `AppConfig.APP_TO_APP_CALLBACK` — сервер рүү илгээх утга
 *   2. `AndroidManifest.xml`-ийн intent-filter — OS энэ аппыг сэрээх нөхцөл
 *   3. Nexus backend-ийн `EID_APP_CALLBACKS` — бүтэн URI
 *   4. RP-ийн `callback_hosts`-д `gerege-nexus://` scheme (сервер тал, admin-аар)
 *
 * Эхний хоёр нь зөрвөл ЮУ Ч алдаа өгөхгүй: eID апп зөв хаягаар буцаах гэж
 * оролдоод OS «ийм зүйл нээх апп алга» гэж чимээгүй хаяна, хүн eID апп дотроо
 * үлдэнэ. Тиймээс тэр хоёрын таарлыг ЭНД барина. Backend env-ийг Go test,
 * RP-ийн allowlist-ийг зөвхөн eID серверийн `NormalizeCallback` мэднэ.
 */
class CallbackContractTest {

    @Test
    fun manifestDeclaresTheCallbackScheme() {
        val uri = AppConfig.APP_TO_APP_CALLBACK          // "gerege-nexus://auth"
        val scheme = uri.substringBefore("://")
        val host = uri.substringAfter("://").substringBefore('/')

        val manifest = File("src/main/AndroidManifest.xml").readText()
        assertTrue(
            "AndroidManifest-д $uri-ийн intent-filter алга — eID апп буцаахад OS энэ аппыг олохгүй",
            manifest.contains("""android:scheme="$scheme"""") &&
                manifest.contains("""android:host="$host""""),
        )
    }

    @Test
    fun eidAppSchemesAreQueryable() {
        // Android 11+ дээр `<queries>` блокгүй бол `resolveActivity` нь eID аппыг
        // ОЛОХГҮЙ бөгөөд нэвтрэлт «апп суугаагүй» гэсэн буруу мөрөөр явна.
        val manifest = File("src/main/AndroidManifest.xml").readText()
        val queries = manifest.substringAfter("<queries>", "").substringBefore("</queries>")
        listOf("geregesmartid", "eidmongolia").forEach { scheme ->
            assertTrue("<queries> дотор $scheme схем алга", queries.contains("""android:scheme="$scheme""""))
        }
    }
}
