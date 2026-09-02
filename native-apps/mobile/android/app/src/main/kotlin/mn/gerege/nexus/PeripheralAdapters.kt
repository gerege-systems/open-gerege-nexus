package mn.gerege.nexus

import android.content.Context
import android.hardware.usb.UsbManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.InetSocketAddress
import java.net.Socket

object PeripheralAdapters {
    suspend fun printNetworkTest(host: String, port: Int) = withContext(Dispatchers.IO) {
        require(host.isNotBlank()) { "Printer host оруулна уу" }
        Socket().use { socket -> socket.connect(InetSocketAddress(host, port), 5_000); socket.getOutputStream().use { it.write(byteArrayOf(0x1b,0x40)); it.write("Gerege Nexus\nNative ESC/POS test\n\n\n".toByteArray()); it.write(byteArrayOf(0x1d,0x56,0x41,0x03)); it.flush() } }
    }
    suspend fun openDrawer(host: String, port: Int, pulseMs: Int) = withContext(Dispatchers.IO) {
        Socket().use { socket -> socket.connect(InetSocketAddress(host,port),5_000); socket.getOutputStream().use { val pulse=(pulseMs/2).coerceIn(1,255).toByte(); it.write(byteArrayOf(0x1b,0x70,0x00,pulse,pulse));it.flush() } }
    }
    suspend fun printRaw(host:String,port:Int,content:ByteArray,cut:Boolean)=withContext(Dispatchers.IO){require(host.isNotBlank()){ "Printer host оруулна уу" };Socket().use{socket->socket.connect(InetSocketAddress(host,port),8_000);socket.getOutputStream().use{stream->stream.write(content);if(cut)stream.write(byteArrayOf(0x1d,0x56,0x41,0x03));stream.flush()}}}
    fun usbSummary(context: Context): String { val devices=(context.getSystemService(Context.USB_SERVICE) as UsbManager).deviceList.values; return if(devices.isEmpty()) "USB төхөөрөмж олдсонгүй" else devices.joinToString { "${it.productName ?: "USB"} (${it.vendorId}:${it.productId})" } }
}
