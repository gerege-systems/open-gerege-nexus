package mn.gerege.nexus.core.device

import kotlinx.coroutines.test.runTest
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import kotlin.test.Test
import kotlin.test.assertEquals

class DeviceEnrollmentApiTest {
    @Test fun enrollReturnsOneTimeTokenAndMeUsesDeviceScheme() = runTest {
        val server = MockWebServer()
        server.enqueue(MockResponse().setBody("""{"device_id":"d1","tenant_id":"t1","device_token":"secret"}"""))
        server.enqueue(MockResponse().setBody("""{"id":"d1","tenant_id":"t1","name":"POS 1","platform":"android","form_factor":"pos","status":"ACTIVE"}"""))
        server.start()
        try {
            val api = DeviceEnrollmentApi(server.url("api/v1/").toString())
            assertEquals("secret", api.enroll("AAAA-BBBB-CCCC-DDDD", "POS 1", "android", "pos", "S1", "1", "Android").token)
            assertEquals("ACTIVE", api.me("secret").status)
            server.takeRequest()
            val identityRequest = server.takeRequest()
            assertEquals("Device secret", identityRequest.getHeader("Authorization"))
        } finally { server.shutdown() }
    }
}
