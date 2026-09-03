using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Certificates;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public partial class VerifyViewModel : ObservableObject
{
    private readonly ILocalizationService _l10n;
    private readonly ICertificateService _certs;
    private readonly IClock _clock;

    public VerifyViewModel(ILocalizationService l10n, ICertificateService certs, IClock clock)
    {
        _l10n = l10n;
        _certs = certs;
        _clock = clock;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private string _subtitle = string.Empty;

    [ObservableProperty]
    private string _dropZoneHint = string.Empty;

    [ObservableProperty]
    private string _pasteHint = string.Empty;

    [ObservableProperty]
    private string _pasteText = string.Empty;

    [ObservableProperty]
    private string _selectedFilePath = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasCertificate))]
    [NotifyPropertyChangedFor(nameof(ValidityVariant))]
    [NotifyPropertyChangedFor(nameof(ValidityLabel))]
    private CertificateInfo? _certificate;

    [ObservableProperty]
    private string _errorMessage = string.Empty;

    [ObservableProperty]
    private bool _isParsing;

    public bool HasCertificate => Certificate is not null;

    public string ValidityVariant
    {
        get
        {
            if (Certificate is null) return "Idle";
            var now = _clock.UtcNow;
            if (Certificate.IsExpired(now)) return "Bad";
            if (Certificate.IsNotYetValid(now)) return "Warn";
            return "OK";
        }
    }

    public string ValidityLabel
    {
        get
        {
            if (Certificate is null) return string.Empty;
            var now = _clock.UtcNow;
            if (Certificate.IsExpired(now)) return _l10n.GetString("Verify_Validity_Expired");
            if (Certificate.IsNotYetValid(now)) return _l10n.GetString("Verify_Validity_NotYetValid");
            return _l10n.GetString("Verify_Validity_Valid");
        }
    }

    [RelayCommand]
    private void ParsePaste()
    {
        if (string.IsNullOrWhiteSpace(PasteText))
        {
            ErrorMessage = _l10n.GetString("Verify_Error_EmptyInput");
            Certificate = null;
            return;
        }
        IsParsing = true;
        try
        {
            var result = _certs.ParsePem(PasteText);
            if (result.IsSuccess)
            {
                Certificate = result.Value;
                ErrorMessage = string.Empty;
            }
            else
            {
                Certificate = null;
                ErrorMessage = result.Error.Message;
            }
        }
        finally
        {
            IsParsing = false;
        }
    }

    public void ParseFromFilePath(string filePath)
    {
        IsParsing = true;
        try
        {
            SelectedFilePath = filePath;
            var result = _certs.ParseFile(filePath);
            if (result.IsSuccess)
            {
                Certificate = result.Value;
                PasteText = result.Value.PemSource;
                ErrorMessage = string.Empty;
            }
            else
            {
                Certificate = null;
                ErrorMessage = result.Error.Message;
            }
        }
        finally
        {
            IsParsing = false;
        }
    }

    [RelayCommand]
    private void Clear()
    {
        Certificate = null;
        PasteText = string.Empty;
        SelectedFilePath = string.Empty;
        ErrorMessage = string.Empty;
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Verify_Title");
        Subtitle = _l10n.GetString("Verify_Subtitle");
        DropZoneHint = _l10n.GetString("Verify_DropZone_Hint");
        PasteHint = _l10n.GetString("Verify_Paste_Hint");
        OnPropertyChanged(nameof(ValidityLabel));
    }
}
