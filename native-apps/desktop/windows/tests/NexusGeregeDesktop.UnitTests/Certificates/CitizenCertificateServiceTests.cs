using System.Net;
using System.Text;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Infrastructure.Certificates;
using Microsoft.Extensions.Logging.Abstractions;
using NSubstitute;

namespace NexusGeregeDesktop.UnitTests.Certificates;

/// `POST /api/certificates` — session-bound wire contract
/// (web/src/app/api/certificates/route.ts + web/src/lib/rpclient.ts).
public class CitizenCertificateServiceTests
{
    private const string SampleBody = """
    {
      "personEtsi": "PNOMN-11011111111",
      "counts": { "valid": 2, "revoked": 1, "expired": 3, "suspended": 0, "total": 6 },
      "certificates": [
        { "documentNumber": "d-auth", "type": "AUTH", "serialNumber": "0A1B",
          "certificateLevel": "QUALIFIED", "status": "VALID",
          "notBefore": "2026-01-05T00:00:00Z", "notAfter": "2029-01-05T00:00:00Z",
          "issuerDn": "CN=eID Mongolia CA, O=NDC, C=MN", "certValue": "QUJD" },
        { "documentNumber": "d-sign", "type": "SIGN", "serialNumber": "0C2D",
          "certificateLevel": "QUALIFIED", "status": "VALID",
          "notBefore": "2026-06-05T00:00:00Z", "notAfter": "2029-06-05T00:00:00Z",
          "issuerDn": "CN=eID Mongolia CA, O=NDC, C=MN", "certValue": "REVG" },
        { "documentNumber": "d-old", "type": "SIGN", "serialNumber": "0E3F",
          "certificateLevel": "", "status": "EXPIRED",
          "notBefore": "", "notAfter": "", "issuerDn": "", "certValue": "" }
      ],
      "signing": "REVG", "auth": "QUJD", "certificateLevel": "QUALIFIED"
    }
    """;

    [Fact]
    public async Task ListAsync_PostsSessionCredentials_AndMapsResponse()
    {
        var handler = new StubHandler(HttpStatusCode.OK, SampleBody);
        var service = Build(handler, Session());

        var result = await service.ListAsync();

        result.IsSuccess.Should().BeTrue();
        handler.LastPath.Should().Be("/api/certificates");
        // Session-bound: personId is deliberately NOT sent (broken-access-control fix).
        handler.LastBody.Should().Contain("\"sessionId\":\"sess-1\"");
        handler.LastBody.Should().Contain("\"pollToken\":\"poll-1\"");
        handler.LastBody.Should().NotContain("personId");

        var list = result.Value;
        list.PersonEtsi.Should().Be("PNOMN-11011111111");
        list.Counts.Valid.Should().Be(2);
        list.Counts.Expired.Should().Be(3);
        list.Counts.Total.Should().Be(6);
        list.Certificates.Should().HaveCount(3);

        // Newest notBefore first — the backend's order is not trusted.
        list.Certificates[0].DocumentNumber.Should().Be("d-sign");
        list.Certificates[0].IsSigning.Should().BeTrue();
        list.Certificates[0].IsValid.Should().BeTrue();
        list.Certificates[0].NotAfter.Should().Be(new DateTimeOffset(2029, 6, 5, 0, 0, 0, TimeSpan.Zero));
        list.Certificates[1].DocumentNumber.Should().Be("d-auth");
    }

    [Fact]
    public async Task ListAsync_EmptyDateStrings_MapToNull_WithoutThrowing()
    {
        var handler = new StubHandler(HttpStatusCode.OK, SampleBody);
        var service = Build(handler, Session());

        var result = await service.ListAsync();

        var stale = result.Value.Certificates.Single(c => c.DocumentNumber == "d-old");
        stale.NotBefore.Should().BeNull();
        stale.NotAfter.Should().BeNull();
        stale.CertValueB64.Should().BeEmpty();
        stale.IsValid.Should().BeFalse();
    }

    [Fact]
    public async Task ListAsync_WithoutSession_FailsUnauthorized_WithoutCallingBackend()
    {
        var handler = new StubHandler(HttpStatusCode.OK, SampleBody);
        var service = Build(handler, session: null);

        var result = await service.ListAsync();

        result.IsFailure.Should().BeTrue();
        result.Error.Code.Should().Be(ApiErrorCode.Unauthorized);
        handler.CallCount.Should().Be(0);
    }

    [Theory]
    [InlineData(HttpStatusCode.Unauthorized, ApiErrorCode.Unauthorized, "unauthenticated")]
    [InlineData(HttpStatusCode.NotFound, ApiErrorCode.Internal, "certificates_backend_not_available")]
    [InlineData((HttpStatusCode)429, ApiErrorCode.BadRequest, "rate_limited")]
    [InlineData(HttpStatusCode.InternalServerError, ApiErrorCode.ServerError, "server_500")]
    public async Task ListAsync_MapsBackendErrors(HttpStatusCode status, ApiErrorCode expectedCode, string expectedMessage)
    {
        var handler = new StubHandler(status, """{"error":"nope"}""");
        var service = Build(handler, Session());

        var result = await service.ListAsync();

        result.IsFailure.Should().BeTrue();
        result.Error.Code.Should().Be(expectedCode);
        result.Error.Message.Should().Be(expectedMessage);
    }

    private static WebSession Session() => new(
        "sess-1",
        Guid.NewGuid(),
        DateTimeOffset.UtcNow.AddHours(1),
        "11011111111",
        "Тест Иргэн",
        "HIGH",
        "poll-1");

    private static CitizenCertificateService Build(StubHandler handler, WebSession? session)
    {
        var sessions = Substitute.For<IWebSessionStore>();
        sessions.Current.Returns(session);
        var http = new HttpClient(handler) { BaseAddress = new Uri("https://eidmongolia.mn") };
        return new CitizenCertificateService(http, sessions, NullLogger<CitizenCertificateService>.Instance);
    }

    private sealed class StubHandler : HttpMessageHandler
    {
        private readonly HttpStatusCode _status;
        private readonly string _body;

        public StubHandler(HttpStatusCode status, string body)
        {
            _status = status;
            _body = body;
        }

        public int CallCount { get; private set; }

        public string? LastPath { get; private set; }

        public string LastBody { get; private set; } = string.Empty;

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            CallCount++;
            LastPath = request.RequestUri?.AbsolutePath;
            if (request.Content is not null)
            {
                LastBody = await request.Content.ReadAsStringAsync(cancellationToken);
            }
            return new HttpResponseMessage(_status)
            {
                Content = new StringContent(_body, Encoding.UTF8, "application/json"),
            };
        }
    }
}
