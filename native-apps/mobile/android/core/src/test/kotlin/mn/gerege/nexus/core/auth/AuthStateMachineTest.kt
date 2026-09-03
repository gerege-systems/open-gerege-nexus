package mn.gerege.nexus.core.auth

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import java.net.UnknownHostException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

@OptIn(ExperimentalCoroutinesApi::class)
class AuthStateMachineTest {
    @Test fun passwordSuccessPublishesSuccessAndKeepsSessionCookie() = runTest {
        val server = MockWebServer()
        server.enqueue(MockResponse().setResponseCode(200)
            .setHeader("Set-Cookie", "session_token=abc; Path=/; HttpOnly")
            .setBody("{}"))
        server.start()
        try {
            val machine = AuthStateMachine(AuthApi(server.url("api/v1/").toString()), this)
            machine.password("admin@example.com", "secret")
            val terminal = machine.phase.first { it !is LoginPhase.Idle && it !is LoginPhase.Starting && it !is LoginPhase.Waiting }
            assertEquals(LoginPhase.Success, terminal)
            assertEquals("abc", machine.sessionCookies().single().value)
            assertEquals("/api/v1/auth/login", server.takeRequest().path)
        } finally { server.shutdown() }
    }

    @Test fun passwordFailurePublishesError() = runTest {
        val server = MockWebServer()
        server.enqueue(MockResponse().setResponseCode(401).setBody("{\"error\":\"invalid credentials\"}"))
        server.start()
        try {
            val machine = AuthStateMachine(AuthApi(server.url("api/v1/").toString()), this)
            machine.password("bad@example.com", "bad")
            val terminal = machine.phase.first { it is LoginPhase.Error }
            assertEquals("invalid credentials", assertIs<LoginPhase.Error>(terminal).message)
        } finally { server.shutdown() }
    }

    // Порт хаагдсан сервер → ConnectException. Хэрэглэгчид Java-гийн англи
    // бичвэр биш, монгол тайлбар харагдах ёстой; техник бичвэр detail-д үлдэнэ.
    @Test fun unreachableServerPublishesHumanReadableError() = runTest {
        val server = MockWebServer()
        server.start()
        val url = server.url("api/v1/").toString()
        server.shutdown()
        val machine = AuthStateMachine(AuthApi(url), this)
        machine.password("admin@example.com", "secret")
        val terminal = assertIs<LoginPhase.Error>(machine.phase.first { it is LoginPhase.Error })
        assertEquals("Сервертэй холбогдож чадсангүй", terminal.message)
        assertTrue(terminal.detail!!.contains("ConnectException"), terminal.detail)
    }

    @Test fun networkFailuresAreDescribedInMongolianButApiMessagesPassThrough() {
        val dns = describeFailure(UnknownHostException("Unable to resolve host \"mobile.nexus.gerege.mn\": No address associated with hostname"))
        assertEquals("Сервертэй холбогдож чадсангүй — интернэт холболтоо шалгана уу", dns.message)
        assertTrue(dns.detail!!.contains("mobile.nexus.gerege.mn"), dns.detail)
        // API-гийн хариу (AuthApi.post → error(...)) серверийн бичвэрээ хэвээр авчирна.
        assertEquals("invalid credentials", describeFailure(IllegalStateException("invalid credentials")).message)
    }
}

