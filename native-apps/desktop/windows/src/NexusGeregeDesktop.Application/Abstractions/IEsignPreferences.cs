namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// ESIGN гүүрийн хэрэглэгчийн сонголт. Анхдагчаар физик USB токен давуу эрхтэй
/// (залгаастай бол түүгээр зурна); энэ тохиргоог асаавал токен залгаастай ч
/// апп-д нэвтэрсэн иргэний программ токеноор (утас руу PIN2 push) зурна.
/// Тохиргоог UI давхарга (Client) хадгална — Windows-ийн LocalSettings.
/// </summary>
public interface IEsignPreferences
{
    bool PreferSoftwareToken { get; }

    event EventHandler<bool>? PreferSoftwareTokenChanged;

    void SetPreferSoftwareToken(bool value);
}
