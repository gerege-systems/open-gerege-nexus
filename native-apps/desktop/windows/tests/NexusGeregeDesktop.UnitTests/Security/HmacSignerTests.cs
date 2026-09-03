using System.Text;
using NexusGeregeDesktop.Infrastructure.Security.Hmac;

namespace NexusGeregeDesktop.UnitTests.Security;

public class HmacSignerTests
{
    /// Cross-language reference vector pinned in:
    ///   backend/internal/platform/server/middleware/hmac_nonce_test.go::TestHMAC_CrossLanguageReferenceVector
    ///   android/.../HmacInterceptorTest.kt::crossLanguageReferenceVector
    ///   ios/Tests/UnitTests/HMACSignerTests.swift::testCrossLanguageReferenceVector
    /// If this fails, the canonical-bytes contract has drifted; mobile + web clients
    /// will start failing with 401 from the server.
    [Fact]
    public void CrossLanguageReferenceVector_MatchesPinnedSignature()
    {
        var secret = new byte[32];
        Array.Fill(secret, (byte)0xFF);

        const string method = "POST";
        const string path = "/mobile/v1/sign/77777777-7777-7777-7777-777777777777/confirm";
        var body = Encoding.UTF8.GetBytes("""{"partial_signature":"deadbeef"}""");
        const string timestamp = "1745749200";
        const string nonce = "fixed-nonce-2026";

        var bodyHash = HmacSigner.HashBody(body);
        var canonical = HmacSigner.BuildCanonical(method, path, bodyHash, timestamp, nonce);
        var signature = HmacSigner.Sign(secret, canonical);

        signature.Should().Be("0988e40afec5611b78cb3517793d6702475e55d62ea007ec6e107ef00b1bde48");
    }

    [Fact]
    public void EmptyBody_Sha256_MatchesPinnedHash()
    {
        var hash = HmacSigner.HashBody(ReadOnlySpan<byte>.Empty);
        Convert.ToHexString(hash).ToLowerInvariant()
            .Should().Be("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
    }

    [Fact]
    public void BuildCanonical_FormatMatchesBackend_OrderedNewlineSeparated()
    {
        var bodyHash = HmacSigner.HashBody("hello"u8);
        var canonical = HmacSigner.BuildCanonical(
            "GET",
            "/web2app/v1/health",
            bodyHash,
            "1700000000",
            "abc-123");

        canonical.Should().Be(
            "GET\n" +
            "/web2app/v1/health\n" +
            "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824\n" +
            "1700000000\n" +
            "abc-123");
    }

    [Fact]
    public void BodyMutation_ChangesSignature()
    {
        var secret = new byte[32];
        Array.Fill(secret, (byte)0x42);

        var sigA = HmacSigner.SignRequest(
            secret, "POST", "/test", "alpha"u8, 1700000000, "n");
        var sigB = HmacSigner.SignRequest(
            secret, "POST", "/test", "alpha "u8, 1700000000, "n");

        sigA.Should().NotBe(sigB);
    }

    [Fact]
    public void Sign_RejectsEmptySecret()
    {
        var act = () => HmacSigner.Sign(ReadOnlySpan<byte>.Empty, "canonical");
        act.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void BuildCanonical_RejectsWrongBodyHashLength()
    {
        var bad = new byte[31];
        var act = () => HmacSigner.BuildCanonical("GET", "/x", bad, "1", "n");
        act.Should().Throw<ArgumentException>().And.ParamName.Should().Be("bodyHash");
    }
}
