using System;
using System.Windows;

namespace GeregeNexusNativeWin
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// Pure Native C# .NET 8 WPF Shell
    /// </summary>
    public partial class App : Application
    {
        protected override void OnStartup(StartupEventArgs e)
        {
            base.OnStartup(e);
            AppDomain.CurrentDomain.UnhandledException += (s, ev) =>
            {
                MessageBox.Show($"Unhandled Error: {ev.ExceptionObject}", "Gerege Nexus Native Error", MessageBoxButton.OK, MessageBoxImage.Error);
            };
        }
    }
}
