using System.Globalization;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Windows.Storage;
using Windows.Storage.Pickers;
using WinRT.Interop;

namespace NexusGeregeDesktop.Client.Controls;

/// Modal dialog that displays the full X.509 cert decoding (subject,
/// issuer, validity, key + signature alg, fingerprints, key usage, SAN,
/// PEM body). Hosted from TokenDetailPage when the citizen clicks a
/// certificate row in the object list — mirrors the "Certificate View"
/// button in EnterSafe PKI Manager.
public sealed partial class CertDetailDialog : ContentDialog
{
    // CA1861 wants these as static readonly rather than inline arrays.
    private static readonly string[] DerExtensions = new[] { ".cer", ".crt", ".der" };
    private static readonly string[] PemExtensions = new[] { ".pem" };

    private byte[]? _der;
    private string? _suggestedFileBaseName;

    public CertDetailDialog()
    {
        InitializeComponent();
    }

    /// Loads the dialog with parsed fields from a DER-encoded cert blob.
    /// Caller is responsible for showing the dialog (await ShowAsync).
    public void LoadFromDer(byte[] der)
    {
        try
        {
            _der = der;
            using var cert = new X509Certificate2(der);
            _suggestedFileBaseName = SuggestFileName(cert);
            Populate(cert);
        }
        catch
        {
            // Defensive — if parse fails we leave the placeholder texts.
        }
    }

    /// Suggest a filename based on the cert's CN. "Test Mongolia Sign"
    /// → "test-mongolia-sign". Strips quotes / pipe / slash chars that
    /// would break on Windows filesystems.
    private static string SuggestFileName(X509Certificate2 cert)
    {
        var cn = cert.GetNameInfo(X509NameType.SimpleName, false);
        if (string.IsNullOrWhiteSpace(cn))
        {
            return $"cert-{cert.SerialNumber}";
        }
        var safe = new StringBuilder(cn.Length);
        foreach (var c in cn)
        {
            if (char.IsLetterOrDigit(c)) safe.Append(char.ToLowerInvariant(c));
            else if (c == ' ' || c == '-' || c == '_') safe.Append('-');
        }
        var s = safe.ToString().Trim('-');
        return string.IsNullOrEmpty(s) ? "cert" : s;
    }

    private async void OnExportClick(ContentDialog sender, ContentDialogButtonClickEventArgs args)
    {
        // Cancel default close so the dialog stays open if the picker
        // is cancelled — citizen can re-try or hit Close manually.
        var deferral = args.GetDeferral();
        try
        {
            if (_der is null || _der.Length == 0)
            {
                return;
            }
            var picker = new FileSavePicker();
            picker.SuggestedStartLocation = PickerLocationId.DocumentsLibrary;
            picker.FileTypeChoices.Add("DER-encoded certificate (binary)", DerExtensions);
            picker.FileTypeChoices.Add("PEM-encoded certificate (text)", PemExtensions);
            picker.SuggestedFileName = _suggestedFileBaseName ?? "cert";

            // WinUI 3 FilePickers need the window handle explicitly —
            // there's no implicit window context for unpackaged-style
            // dialogs. App.WindowHandle is set by App.OnLaunched.
            var hwnd = App.WindowHandle;
            if (hwnd != IntPtr.Zero)
            {
                InitializeWithWindow.Initialize(picker, hwnd);
            }

            var file = await picker.PickSaveFileAsync();
            if (file is null) return;

            // Write DER or PEM based on the chosen extension.
            var ext = Path.GetExtension(file.Name).ToLowerInvariant();
            byte[] bytes = ext == ".pem"
                ? Encoding.ASCII.GetBytes(PemText.Text)
                : _der;
            await FileIO.WriteBytesAsync(file, bytes);
        }
        catch
        {
            // Best-effort — picker exceptions / disk-full / etc. are
            // silently swallowed; citizen can re-open the dialog and
            // try again.
        }
        finally
        {
            // Keep dialog open after a failed/cancelled save — make
            // 'Close' the explicit dismissal. Returning args.Cancel
            // doesn't keep the dialog open for SecondaryButton in
            // WinUI 3 ContentDialog, so we use Hide+ReShow if we
            // wanted to stay open — but actually the dialog stays
            // open until Primary/Secondary returns from the click
            // handler. Once we exit, the secondary button DOES
            // dismiss the dialog by default. Set Cancel=true to keep
            // it visible after Export so user can verify the file.
            args.Cancel = true;
            deferral.Complete();
        }
    }


    private void Populate(X509Certificate2 cert)
    {
        SubjectText.Text = cert.Subject;
        IssuerText.Text = cert.Issuer;
        NotBeforeText.Text = cert.NotBefore.ToString("yyyy-MM-dd HH:mm:ss zzz", CultureInfo.InvariantCulture);
        NotAfterText.Text = cert.NotAfter.ToString("yyyy-MM-dd HH:mm:ss zzz", CultureInfo.InvariantCulture);

        PublicKeyText.Text = FormatPublicKey(cert);
        SignatureAlgText.Text = cert.SignatureAlgorithm.FriendlyName ?? cert.SignatureAlgorithm.Value ?? "?";
        SerialText.Text = cert.SerialNumber;
        Sha256Text.Text = ColonHex(SHA256.HashData(cert.RawData));
        // SHA-1 fingerprint is a long-standing X.509 identifier (every
        // cert viewer in the world shows it) — analyzer CA5350 flags
        // it as "weak crypto" but we're not using it as a security
        // primitive, just as a display hash.
#pragma warning disable CA5350
        Sha1Text.Text = ColonHex(SHA1.HashData(cert.RawData));
#pragma warning restore CA5350
        KeyUsageText.Text = FormatKeyUsage(cert);

        var ekus = FormatExtendedKeyUsage(cert);
        if (!string.IsNullOrEmpty(ekus))
        {
            ExtendedKeyUsagePanel.Visibility = Visibility.Visible;
            ExtendedKeyUsageText.Text = ekus;
        }

        var sans = FormatSubjectAlternativeNames(cert);
        if (!string.IsNullOrEmpty(sans))
        {
            SanPanel.Visibility = Visibility.Visible;
            SanText.Text = sans;
        }

        PemText.Text = DerToPem(cert.RawData);
    }

    private static string FormatPublicKey(X509Certificate2 cert)
    {
        switch (cert.GetRSAPublicKey())
        {
            case { } rsa:
                return $"RSA · {rsa.KeySize} bit";
        }
        switch (cert.GetECDsaPublicKey())
        {
            case { } ec:
                return $"ECDSA · {ec.KeySize} bit";
        }
        return cert.PublicKey.Oid?.FriendlyName ?? cert.PublicKey.Oid?.Value ?? "?";
    }

    private static string FormatKeyUsage(X509Certificate2 cert)
    {
        foreach (var ext in cert.Extensions)
        {
            if (ext is X509KeyUsageExtension ku)
            {
                return ku.KeyUsages.ToString();
            }
        }
        return "—";
    }

    private static string FormatExtendedKeyUsage(X509Certificate2 cert)
    {
        foreach (var ext in cert.Extensions)
        {
            if (ext is X509EnhancedKeyUsageExtension eku)
            {
                var parts = new List<string>();
                foreach (var oid in eku.EnhancedKeyUsages)
                {
                    parts.Add(oid.FriendlyName ?? oid.Value ?? "?");
                }
                return string.Join(", ", parts);
            }
        }
        return string.Empty;
    }

    private static string FormatSubjectAlternativeNames(X509Certificate2 cert)
    {
        foreach (var ext in cert.Extensions)
        {
            if (ext.Oid?.Value == "2.5.29.17")
            {
                // SAN OID — print FormattedData which is human-readable
                // (DNS:, URI:, IP:, etc.). The .NET API returns one
                // string with newlines between entries.
                return ext.Format(true);
            }
        }
        return string.Empty;
    }

    private static string ColonHex(byte[] bytes)
    {
        const string hex = "0123456789ABCDEF";
        if (bytes.Length == 0) return string.Empty;
        var sb = new StringBuilder(bytes.Length * 3 - 1);
        for (var i = 0; i < bytes.Length; i++)
        {
            if (i > 0) sb.Append(':');
            sb.Append(hex[bytes[i] >> 4]);
            sb.Append(hex[bytes[i] & 0x0F]);
        }
        return sb.ToString();
    }

    private static string DerToPem(byte[] der)
    {
        var b64 = Convert.ToBase64String(der);
        var sb = new StringBuilder(b64.Length + 64);
        sb.Append("-----BEGIN CERTIFICATE-----\n");
        for (var i = 0; i < b64.Length; i += 64)
        {
            sb.Append(b64.AsSpan(i, Math.Min(64, b64.Length - i)));
            sb.Append('\n');
        }
        sb.Append("-----END CERTIFICATE-----\n");
        return sb.ToString();
    }
}
