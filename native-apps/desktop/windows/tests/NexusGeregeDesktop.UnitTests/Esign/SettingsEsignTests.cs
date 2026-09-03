using System.Globalization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Presentation.Services;
using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using NSubstitute;

namespace NexusGeregeDesktop.UnitTests.Esign;

/// Тохиргоо хуудсын ESIGN хэсэг — программ токены сонголт + гүүрийг асаах/зогсоох.
public class SettingsEsignTests
{
    [Fact]
    public void PreferSoftwareToken_InitializesFromPreferences()
    {
        var prefs = Substitute.For<IEsignPreferences>();
        prefs.PreferSoftwareToken.Returns(true);

        var vm = Build(prefs, Substitute.For<IEsignBridgeService>());

        vm.PreferSoftwareToken.Should().BeTrue();
    }

    [Fact]
    public void TogglingPreference_PersistsThroughPreferencesService()
    {
        var prefs = Substitute.For<IEsignPreferences>();
        prefs.PreferSoftwareToken.Returns(false);
        var vm = Build(prefs, Substitute.For<IEsignBridgeService>());

        vm.PreferSoftwareToken = true;

        prefs.Received(1).SetPreferSoftwareToken(true);
    }

    [Fact]
    public void ToggleEsignBridge_StartsWhenStopped_AndStopsWhenRunning()
    {
        var bridge = Substitute.For<IEsignBridgeService>();
        bridge.IsRunning.Returns(false);
        var vm = Build(Substitute.For<IEsignPreferences>(), bridge);

        vm.ToggleEsignBridgeCommand.Execute(null);
        bridge.Received(1).Start();

        bridge.IsRunning.Returns(true);
        vm.ToggleEsignBridgeCommand.Execute(null);
        bridge.Received(1).Stop();
    }

    [Fact]
    public void EsignStatus_ReportsBothPorts_WhenRunning()
    {
        var bridge = Substitute.For<IEsignBridgeService>();
        bridge.IsRunning.Returns(true);
        bridge.WsPort.Returns(59001);

        var vm = Build(Substitute.For<IEsignPreferences>(), bridge);

        // Локалчлагдсан формат мөр нь ws + wss (порт + 4) хоёуланг агуулна.
        vm.EsignStatus.Should().Contain("59001").And.Contain("59005");
    }

    private static SettingsViewModel Build(IEsignPreferences prefs, IEsignBridgeService bridge)
    {
        var mn = CultureInfo.GetCultureInfo("mn-MN");

        var l10n = Substitute.For<ILocalizationService>();
        l10n.Current.Returns(mn);
        l10n.Supported.Returns(new[] { mn });
        // Түлхүүрийг өөрийг нь буцаана — формат мөр нь {0}/{1}-тэй байх ёстой тул
        // гүүрийн төлөвийн түлхүүрт бодит загварыг өгнө.
        l10n.GetString(Arg.Any<string>()).Returns(call => call.Arg<string>() switch
        {
            "Settings_Esign_Bridge_Running" => "ws:{0} / wss:{1}",
            var key => key,
        });

        var theme = Substitute.For<IThemeService>();
        theme.Current.Returns(AppTheme.System);

        var tokens = Substitute.For<ITokenRegistry>();
        tokens.Providers.Returns(Array.Empty<ICryptoTokenProvider>());

        var ui = Substitute.For<IUiDispatcher>();
        ui.When(x => x.Post(Arg.Any<Action>())).Do(call => call.Arg<Action>()());

        return new SettingsViewModel(theme, l10n, tokens, prefs, bridge, ui);
    }
}
