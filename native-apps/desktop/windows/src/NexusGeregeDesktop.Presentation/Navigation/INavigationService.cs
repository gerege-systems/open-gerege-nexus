namespace NexusGeregeDesktop.Presentation.Navigation;

public interface INavigationService
{
    bool CanGoBack { get; }

    string? CurrentRoute { get; }

    event EventHandler<string>? Navigated;

    bool NavigateTo(string routeKey, object? parameter = null);

    bool GoBack();
}
