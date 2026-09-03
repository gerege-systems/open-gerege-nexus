using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Infrastructure.Security.WindowsHello;
using Microsoft.Extensions.Logging.Abstractions;
using Microsoft.Extensions.Options;
using NSubstitute;

namespace NexusGeregeDesktop.UnitTests.Security;

/// Гарын үсэг зурахын өмнөх Windows Hello баталгааны БОДЛОГО:
/// Hello байхгүй төхөөрөмж дээр нэвтрүүлнэ (fail-open), харин Hello байгаа
/// мөртлөө хэрэглэгч цуцалсан/амжилтгүй бол зогсооно (fail-closed).
public class SensitiveActionGuardTests
{
    [Fact]
    public async Task WhenNotRequired_SkipsHelloEntirely()
    {
        var hello = Substitute.For<IWindowsHello>();
        var guard = Build(hello, required: false);

        var result = await guard.RequireConsentAsync("reason", "sign.pdf");

        result.IsSuccess.Should().BeTrue();
        await hello.DidNotReceiveWithAnyArgs().RequestConsentAsync(default!, default);
    }

    [Fact]
    public async Task Verified_Allows_AndAudits()
    {
        var audit = Substitute.For<IAuditLog>();
        var guard = Build(Hello(HelloResult.Verified), required: true, audit: audit);

        var result = await guard.RequireConsentAsync("reason", "sign.pdf");

        result.IsSuccess.Should().BeTrue();
        audit.Received(1).Write(Arg.Is<AuditEvent>(e => e.Kind == AuditEvents.ConsentGranted));
    }

    [Theory]
    [InlineData(HelloResult.NotAvailable)]
    [InlineData(HelloResult.DeviceNotPresent)]
    [InlineData(HelloResult.DisabledByPolicy)]
    public async Task WhenHelloUnavailableOnDevice_AllowsSoSigningIsNotBricked(HelloResult outcome)
    {
        var audit = Substitute.For<IAuditLog>();
        var guard = Build(Hello(outcome), required: true, audit: audit);

        var result = await guard.RequireConsentAsync("reason", "sign.esign");

        result.IsSuccess.Should().BeTrue();
        audit.Received(1).Write(Arg.Is<AuditEvent>(e => e.Kind == AuditEvents.ConsentUnavailable));
    }

    [Theory]
    [InlineData(HelloResult.Cancelled, "hello_cancelled")]
    [InlineData(HelloResult.RetriesExhausted, "hello_failed")]
    [InlineData(HelloResult.Failed, "hello_failed")]
    public async Task WhenCitizenDeclines_Blocks_AndAudits(HelloResult outcome, string expectedMessage)
    {
        var audit = Substitute.For<IAuditLog>();
        var guard = Build(Hello(outcome), required: true, audit: audit);

        var result = await guard.RequireConsentAsync("reason", "sign.pdf");

        result.IsFailure.Should().BeTrue();
        result.Error.Code.Should().Be(ApiErrorCode.Unauthorized);
        result.Error.Message.Should().Be(expectedMessage);
        audit.Received(1).Write(Arg.Is<AuditEvent>(e => e.Kind == AuditEvents.ConsentDenied));
    }

    private static IWindowsHello Hello(HelloResult outcome)
    {
        var hello = Substitute.For<IWindowsHello>();
        hello.RequestConsentAsync(Arg.Any<string>(), Arg.Any<CancellationToken>())
             .Returns(new ValueTask<HelloResult>(outcome));
        return hello;
    }

    private static SensitiveActionGuard Build(IWindowsHello hello, bool required, IAuditLog? audit = null)
    {
        var options = Options.Create(new NexusGeregeDesktopOptions
        {
            Security = new SecurityOptions { RequireWindowsHello = required },
        });
        var clock = Substitute.For<IClock>();
        clock.UtcNow.Returns(DateTimeOffset.UnixEpoch);
        return new SensitiveActionGuard(
            hello,
            audit ?? Substitute.For<IAuditLog>(),
            clock,
            options,
            NullLogger<SensitiveActionGuard>.Instance);
    }
}
