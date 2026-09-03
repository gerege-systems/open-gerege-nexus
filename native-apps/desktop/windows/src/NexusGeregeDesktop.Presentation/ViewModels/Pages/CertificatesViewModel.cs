using System.Collections.ObjectModel;
using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Certificates;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

/// <summary>
/// "Миний гэрчилгээ" — нэвтэрсэн иргэний PKI гэрчилгээний жагсаалт
/// (`POST /api/certificates`, session-bound). Мөр дээр дарахад DER-ийг
/// CertDetailDialog задалж бүрэн X.509 харуулна.
/// </summary>
public partial class CertificatesViewModel : ObservableObject
{
    private readonly ILocalizationService _l10n;
    private readonly ICitizenCertificateService _certificates;
    private readonly IWebSessionStore _sessions;
    private readonly INavigationService _navigation;
    private readonly IUiDispatcher _ui;

    public CertificatesViewModel(
        ILocalizationService l10n,
        ICitizenCertificateService certificates,
        IWebSessionStore sessions,
        INavigationService navigation,
        IUiDispatcher ui)
    {
        _l10n = l10n;
        _certificates = certificates;
        _sessions = sessions;
        _navigation = navigation;
        _ui = ui;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _refreshLabel = string.Empty;
    [ObservableProperty] private string _emptyLabel = string.Empty;
    [ObservableProperty] private string _countsValidLabel = string.Empty;
    [ObservableProperty] private string _countsExpiredLabel = string.Empty;
    [ObservableProperty] private string _countsRevokedLabel = string.Empty;
    [ObservableProperty] private string _countsTotalLabel = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;

    #endregion

    [ObservableProperty] private bool _isLoading;

    [ObservableProperty] private string _countsValid = "0";
    [ObservableProperty] private string _countsExpired = "0";
    [ObservableProperty] private string _countsRevoked = "0";
    [ObservableProperty] private string _countsTotal = "0";

    public ObservableCollection<CertificateRow> Items { get; } = new();

    [RelayCommand]
    private async Task LoadAsync(CancellationToken ct)
    {
        if (_sessions.Current is null)
        {
            _navigation.NavigateTo(NavigationRoutes.Login);
            return;
        }

        IsLoading = true;
        try
        {
            var result = await _certificates.ListAsync(ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                Items.Clear();
                if (result.IsFailure)
                {
                    CountsValid = CountsExpired = CountsRevoked = CountsTotal = "0";
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }

                var list = result.Value;
                CountsValid = list.Counts.Valid.ToString(CultureInfo.InvariantCulture);
                CountsExpired = list.Counts.Expired.ToString(CultureInfo.InvariantCulture);
                CountsRevoked = list.Counts.Revoked.ToString(CultureInfo.InvariantCulture);
                CountsTotal = list.Counts.Total.ToString(CultureInfo.InvariantCulture);
                ErrorMessage = string.Empty;
                foreach (var c in list.Certificates)
                {
                    Items.Add(MapRow(c));
                }
            });
        }
        finally
        {
            _ui.Post(() => IsLoading = false);
        }
    }

    private CertificateRow MapRow(CitizenCertificate c) => new(
        c.DocumentNumber,
        TypeLabel(c.Type),
        c.SerialNumber,
        string.IsNullOrWhiteSpace(c.CertificateLevel) ? "—" : c.CertificateLevel,
        StatusLabel(c.Status),
        StatusVariant(c.Status),
        Validity(c.NotBefore, c.NotAfter),
        IssuerCommonName(c.IssuerDn),
        c.CertValueB64,
        !string.IsNullOrWhiteSpace(c.CertValueB64));

    private string TypeLabel(string type) => type.ToUpperInvariant() switch
    {
        "SIGN" => _l10n.GetString("Certificates_Type_Sign"),
        "AUTH" => _l10n.GetString("Certificates_Type_Auth"),
        _ => string.IsNullOrWhiteSpace(type) ? "—" : type,
    };

    private string StatusLabel(string status) => status.ToUpperInvariant() switch
    {
        "VALID"     => _l10n.GetString("Certificates_Status_Valid"),
        "REVOKED"   => _l10n.GetString("Certificates_Status_Revoked"),
        "EXPIRED"   => _l10n.GetString("Certificates_Status_Expired"),
        "SUSPENDED" => _l10n.GetString("Certificates_Status_Suspended"),
        _ => string.IsNullOrWhiteSpace(status) ? "—" : status,
    };

    // AppCard-ийн StatusVariantBrushConverter-ийн хүлээж авдаг түлхүүрүүд.
    private static string StatusVariant(string status) => status.ToUpperInvariant() switch
    {
        "VALID" => "OK",
        "SUSPENDED" => "Warn",
        _ => "Bad",
    };

    private static string Validity(DateTimeOffset? from, DateTimeOffset? to)
    {
        var a = from is { } f ? f.LocalDateTime.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) : "—";
        var b = to is { } t ? t.LocalDateTime.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) : "—";
        return $"{a} → {b}";
    }

    /// Issuer DN-ээс зөвхөн CN-ийг гаргана ("CN=eID Mongolia CA, O=…" → "eID Mongolia CA").
    /// CN олдоогүй бол бүтэн DN-ийг хэвээр харуулна.
    private static string IssuerCommonName(string dn)
    {
        if (string.IsNullOrWhiteSpace(dn))
        {
            return "—";
        }
        foreach (var part in dn.Split(','))
        {
            var p = part.Trim();
            if (p.StartsWith("CN=", StringComparison.OrdinalIgnoreCase))
            {
                return p[3..].Trim();
            }
        }
        return dn;
    }

    private string TranslateError(ApiError err) => err.Message switch
    {
        "certificates_backend_not_available" => _l10n.GetString("Certificates_Error_NotAvailable"),
        "rate_limited" => _l10n.GetString("Certificates_Error_RateLimited"),
        "unauthenticated" => _l10n.GetString("Certificates_Error_Unauthenticated"),
        null or "" => _l10n.GetString("Certificates_Error_Generic"),
        _ => err.Message,
    };

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Certificates_Title");
        Subtitle = _l10n.GetString("Certificates_Subtitle");
        RefreshLabel = _l10n.GetString("Certificates_Refresh");
        EmptyLabel = _l10n.GetString("Certificates_Empty");
        CountsValidLabel = _l10n.GetString("Certificates_Counts_Valid");
        CountsExpiredLabel = _l10n.GetString("Certificates_Counts_Expired");
        CountsRevokedLabel = _l10n.GetString("Certificates_Counts_Revoked");
        CountsTotalLabel = _l10n.GetString("Certificates_Counts_Total");
    }
}

public sealed record CertificateRow(
    string DocumentNumber,
    string TypeLabel,
    string SerialNumber,
    string Level,
    string StatusLabel,
    string StatusVariant,
    string Validity,
    string Issuer,
    string CertValueB64,
    bool HasDer);
