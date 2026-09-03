// Интернэт байна уу — зөвхөн «сүлжээнд холбогдсон» биш.
//
// Wi-Fi-д холбогдсон ч гарц нь гадагш явуулдаггүй тохиолдол энгийн: captive
// portal, тасарсан uplink, DNS хариулдаггүй router. Тэр үед activeNetwork нь
// null биш хэвээр тул түүгээр шүүвэл pill «Холбогдсон» гэж хуурч, нэвтрэх
// дэлгэц «Unable to resolve host» гэж гэнэт унана. Android өөрөө холболтын
// probe хийж NET_CAPABILITY_VALIDATED тэмдэг тавьдаг — энд түүнийг л уншина.
// Top bar-ын pill ба нэвтрэх дэлгэцийн анхааруулга хоёулаа нэг эх сурвалжтай.

package mn.gerege.nexus

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext

@Composable
internal fun rememberOnline(): Boolean {
    val context = LocalContext.current
    // Эхний утга true: probe-ийн хариу ирэхээс өмнө «интернэтгүй» гэж
    // анивчуулахгүй. Бодит төлөв DisposableEffect дотор шууд тавигдана.
    var online by remember { mutableStateOf(true) }
    DisposableEffect(Unit) {
        val manager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        fun validated(network: Network?): Boolean = network != null &&
            manager.getNetworkCapabilities(network)?.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) == true
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) { online = validated(network) }
            override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) {
                online = capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)
            }
            override fun onLost(network: Network) { online = validated(manager.activeNetwork) }
        }
        online = validated(manager.activeNetwork)
        runCatching { manager.registerDefaultNetworkCallback(callback) }
        onDispose { runCatching { manager.unregisterNetworkCallback(callback) } }
    }
    return online
}
