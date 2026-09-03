using System.Net.Security;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using NexusGeregeDesktop.Infrastructure.Security.Tls;

namespace NexusGeregeDesktop.UnitTests.Security;

/// TLS pinning — appsettings.json нь eidmongolia.mn-ий гинжний ҮНДЭС (ISRG Root X1/X2)-ийг
/// pin хийдэг: leaf нь 60–90 хоног тутам сольдог тул leaf pin апп-ыг тогтмол унагаана.
public class SpkiPinValidatorTests
{
    [Fact]
    public void MatchesChainElement_NotJustLeaf()
    {
        using var root = SelfSigned("CN=Test Root");
        using var leaf = SelfSigned("CN=Test Leaf");
        var validator = new SpkiPinValidator(new[] { "sha256/" + Pin(root) });

        using var chain = new X509Chain();
        chain.ChainPolicy.ExtraStore.Add(root);
        chain.ChainPolicy.VerificationFlags = X509VerificationFlags.AllFlags;
        chain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
        chain.Build(root);

        // Leaf нь pin-д тохирохгүй ч гинжин дэх root тохирвол хүчинтэй.
        validator.Validate(leaf, chain, SslPolicyErrors.None).Should().BeTrue();
    }

    [Fact]
    public void UnknownPin_IsRejected()
    {
        using var leaf = SelfSigned("CN=Test Leaf");
        var validator = new SpkiPinValidator(new[] { "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" });

        validator.Validate(leaf, chain: null, SslPolicyErrors.None).Should().BeFalse();
    }

    [Fact]
    public void TlsErrors_AreFatal_EvenWhenPinMatches()
    {
        using var leaf = SelfSigned("CN=Test Leaf");
        var validator = new SpkiPinValidator(new[] { Pin(leaf) });

        validator.Validate(leaf, chain: null, SslPolicyErrors.RemoteCertificateNameMismatch)
                 .Should().BeFalse();
    }

    [Fact]
    public void NoPinsConfigured_AllowsAnyValidChain()
    {
        using var leaf = SelfSigned("CN=Test Leaf");
        var validator = new SpkiPinValidator(Array.Empty<string>());

        validator.HasPins.Should().BeFalse();
        validator.Validate(leaf, chain: null, SslPolicyErrors.None).Should().BeTrue();
    }

    [Theory]
    [InlineData("sha256/diGVwiVYbubAI3RW4hB9xU8e/CH2GnkuvVFZE8zmgzI=")]  // ISRG Root X2
    [InlineData("sha256/C5+lpZ7tcVwmwQIMcRtPbsQtWLABXhQzejna0wHFr8M=")]  // ISRG Root X1
    public void ShippedPins_AreWellFormedSha256(string pin)
    {
        // Буруу бичсэн pin нь production-д БҮХ холболтыг таслах тул хэлбэрийг барина.
        var raw = pin["sha256/".Length..];
        Convert.FromBase64String(raw).Should().HaveCount(32);
    }

    private static string Pin(X509Certificate2 cert) =>
        Convert.ToBase64String(SHA256.HashData(cert.PublicKey.ExportSubjectPublicKeyInfo()));

    private static X509Certificate2 SelfSigned(string subject)
    {
        using var rsa = RSA.Create(2048);
        var req = new CertificateRequest(subject, rsa, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
        return req.CreateSelfSigned(DateTimeOffset.UtcNow.AddDays(-1), DateTimeOffset.UtcNow.AddDays(1));
    }
}
