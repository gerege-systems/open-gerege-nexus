namespace NexusGeregeDesktop.Presentation.Services;

public interface IUiDispatcher
{
    void Post(Action action);

    bool HasThreadAccess { get; }
}
