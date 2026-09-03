namespace NexusGeregeDesktop.Application.Abstractions;

public interface IClipboardService
{
    /// Copy text to the system clipboard.
    /// When sensitive is true the text is wiped after ClipboardClearSeconds
    /// to limit screen-scraper / shoulder-surfing exposure.
    void SetText(string text, bool sensitive = false);

    void Clear();
}
