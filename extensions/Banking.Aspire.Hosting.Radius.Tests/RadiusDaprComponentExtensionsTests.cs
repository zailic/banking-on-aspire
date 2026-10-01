using CommunityToolkit.Aspire.Hosting.Dapr;
using Xunit;

namespace Banking.Aspire.Hosting.Radius.Tests;

public sealed class RadiusDaprComponentExtensionsTests
{
    [Fact]
    public void CompatibilityResourceMatchesRadiusPreviewMapping()
    {
        IDaprComponentResource resource = new DaprPubSubResource("pubsub");

        Assert.Equal("pubsub", resource.Name);
        Assert.Equal("pubsub", resource.Type);
        Assert.Equal("DaprPubSubResource", resource.GetType().Name);
    }
}
