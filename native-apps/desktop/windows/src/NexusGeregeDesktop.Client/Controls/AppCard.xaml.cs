using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace NexusGeregeDesktop.Client.Controls;

public sealed partial class AppCard : UserControl
{
    public static readonly DependencyProperty BodyProperty =
        DependencyProperty.Register(
            nameof(Body),
            typeof(object),
            typeof(AppCard),
            new PropertyMetadata(null));

    public AppCard() => InitializeComponent();

    public object Body
    {
        get => GetValue(BodyProperty);
        set => SetValue(BodyProperty, value);
    }
}
