using System.Diagnostics.CodeAnalysis;

namespace NexusGeregeDesktop.Domain.Identity;

/// Backend-тэй ижил формат: UUID v4 string-тэй hyphen.
public readonly record struct DeviceId
{
    public Guid Value { get; }

    private DeviceId(Guid value) => Value = value;

    public static DeviceId NewRandom() => new(Guid.NewGuid());

    public static DeviceId From(Guid value)
    {
        if (value == Guid.Empty)
        {
            throw new ArgumentException("DeviceId cannot be empty.", nameof(value));
        }
        return new DeviceId(value);
    }

    public static bool TryParse(string? input, [NotNullWhen(true)] out DeviceId? result)
    {
        if (!string.IsNullOrWhiteSpace(input) && Guid.TryParse(input, out var guid) && guid != Guid.Empty)
        {
            result = new DeviceId(guid);
            return true;
        }
        result = null;
        return false;
    }

    public override string ToString() => Value.ToString("D");
}
