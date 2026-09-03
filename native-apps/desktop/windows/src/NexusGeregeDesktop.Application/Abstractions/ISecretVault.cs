namespace NexusGeregeDesktop.Application.Abstractions;

public interface ISecretVault
{
    ValueTask<byte[]?> ReadAsync(string key, CancellationToken ct = default);

    ValueTask WriteAsync(string key, byte[] value, CancellationToken ct = default);

    ValueTask DeleteAsync(string key, CancellationToken ct = default);

    ValueTask<bool> ExistsAsync(string key, CancellationToken ct = default);
}
