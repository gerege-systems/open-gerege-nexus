using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace NexusGeregeDesktop.Client.Services;

/// <summary>
/// ESIGN гүүрийн хэрэглэгчийн харилцаа — WinUI ContentDialog-оор. Гэрчилгээ сонгох,
/// PIN асуух цонхнуудыг UI thread дээр гаргана.
/// </summary>
public sealed class EsignInteraction : IEsignInteraction
{
    public Task<int> SelectCertificateAsync(IReadOnlyList<EsignCertOption> options, CancellationToken ct = default)
        => RunOnUiAsync(async () =>
        {
            var list = new ListView
            {
                SelectionMode = ListViewSelectionMode.Single,
                MinWidth = 460,
                MaxHeight = 280,
            };
            foreach (var o in options)
            {
                string regno = string.IsNullOrEmpty(o.PersonRegno)
                    ? (string.IsNullOrEmpty(o.OrgRegno) ? "—" : o.OrgRegno)
                    : o.PersonRegno;
                var panel = new StackPanel { Padding = new Thickness(4) };
                panel.Children.Add(new TextBlock { Text = o.OwnerName, FontWeight = Microsoft.UI.Text.FontWeights.SemiBold });
                panel.Children.Add(new TextBlock
                {
                    Text = $"РД: {regno}    ·    {o.Usage}    ·    Хүчинтэй: {o.NotAfter:yyyy-MM-dd}"
                           + (string.IsNullOrEmpty(o.OrgName) ? "" : $"    ·    {o.OrgName}"),
                    Opacity = 0.75,
                    FontSize = 12,
                });
                list.Items.Add(new ListViewItem { Content = panel });
            }
            if (list.Items.Count > 0) list.SelectedIndex = 0;

            var dialog = new ContentDialog
            {
                Title = "Гарын үсэг зурах гэрчилгээ сонгох",
                Content = list,
                PrimaryButtonText = "Сонгох",
                CloseButtonText = "Болих",
                DefaultButton = ContentDialogButton.Primary,
                XamlRoot = App.Window.Content.XamlRoot,
            };
            var res = await dialog.ShowAsync();
            return res == ContentDialogResult.Primary ? Math.Max(0, list.SelectedIndex) : -1;
        });

    public Task<string?> GetPinAsync(string tokenLabel, CancellationToken ct = default)
        => RunOnUiAsync<string?>(async () =>
        {
            var box = new PasswordBox { PlaceholderText = "PIN", MinWidth = 260 };
            var panel = new StackPanel { Spacing = 8 };
            panel.Children.Add(new TextBlock { Text = $"Токен: {tokenLabel}", Opacity = 0.75 });
            panel.Children.Add(box);

            var dialog = new ContentDialog
            {
                Title = "Токены PIN оруулах",
                Content = panel,
                PrimaryButtonText = "Зөвшөөрөх",
                CloseButtonText = "Болих",
                DefaultButton = ContentDialogButton.Primary,
                XamlRoot = App.Window.Content.XamlRoot,
            };
            var res = await dialog.ShowAsync();
            return res == ContentDialogResult.Primary && box.Password.Length > 0 ? box.Password : null;
        });

    public Task<IAsyncDisposable> BeginPushAsync(string verificationCode, CancellationToken ct = default)
        => RunOnUiAsync<IAsyncDisposable>(() =>
        {
            var panel = new StackPanel { Spacing = 12, MinWidth = 300 };
            panel.Children.Add(new TextBlock
            {
                Text = "Гар утсан дээрх eID Mongolia аппаараа баталгаажуулна уу.",
                TextWrapping = TextWrapping.Wrap,
            });
            panel.Children.Add(new TextBlock { Text = "Баталгаажих код:", Opacity = 0.75 });
            panel.Children.Add(new TextBlock
            {
                Text = verificationCode,
                FontSize = 28,
                FontWeight = Microsoft.UI.Text.FontWeights.SemiBold,
                HorizontalAlignment = HorizontalAlignment.Center,
            });
            panel.Children.Add(new ProgressRing { IsActive = true, Width = 32, Height = 32 });

            var dialog = new ContentDialog
            {
                Title = "Гар утсаараа баталгаажуулна уу",
                Content = panel,
                CloseButtonText = "Болих",
                XamlRoot = App.Window.Content.XamlRoot,
            };
            _ = dialog.ShowAsync();   // асаана, хүлээхгүй — гарын үсэг дуусмагц Hide() хийнэ
            return Task.FromResult<IAsyncDisposable>(new DialogCloser(dialog));
        });

    /// <summary>ShowAsync-аар нээсэн цонхыг UI thread дээр хаах disposable.</summary>
    private sealed class DialogCloser : IAsyncDisposable
    {
        private readonly ContentDialog _dialog;
        public DialogCloser(ContentDialog dialog) => _dialog = dialog;
        public ValueTask DisposeAsync()
        {
            var tcs = new TaskCompletionSource();
            bool queued = App.DispatcherQueue.TryEnqueue(() =>
            {
                try { _dialog.Hide(); } finally { tcs.TrySetResult(); }
            });
            if (!queued) tcs.TrySetResult();
            return new ValueTask(tcs.Task);
        }
    }

    private static Task<T> RunOnUiAsync<T>(Func<Task<T>> action)
    {
        var tcs = new TaskCompletionSource<T>();
        bool queued = App.DispatcherQueue.TryEnqueue(async () =>
        {
            try { tcs.SetResult(await action()); }
            catch (Exception ex) { tcs.SetException(ex); }
        });
        if (!queued) tcs.SetException(new InvalidOperationException("UI dispatcher боломжгүй."));
        return tcs.Task;
    }
}
