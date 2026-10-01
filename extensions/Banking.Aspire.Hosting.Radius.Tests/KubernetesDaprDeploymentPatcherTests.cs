using System.Text.Json.Nodes;
using Banking.Aspire.Hosting.Radius;
using Xunit;

namespace Banking.Aspire.Hosting.Radius.Tests;

public sealed class KubernetesDaprDeploymentPatcherTests
{
    [Fact]
    public void BuildMirroredComponentManifestKeepsSpecAndChangesNamespace()
    {
        var source = JsonNode.Parse("""
            {
              "apiVersion": "dapr.io/v1alpha1",
              "kind": "Component",
              "metadata": { "name": "pubsub", "namespace": "default-app" },
              "spec": {
                "type": "pubsub.redis",
                "version": "v1",
                "metadata": [{ "name": "redisHost", "value": "redis.default-app:6379" }]
              }
            }
            """)!.AsObject();

        var result = JsonNode.Parse(
            KubernetesDaprDeploymentPatcher.BuildMirroredComponentManifest(source, "default"))!.AsObject();

        Assert.Equal("default", result["metadata"]!["namespace"]!.GetValue<string>());
        Assert.Equal("pubsub.redis", result["spec"]!["type"]!.GetValue<string>());
        Assert.Equal(
            "redis.default-app:6379",
            result["spec"]!["metadata"]![0]!["value"]!.GetValue<string>());
        Assert.Null(result["metadata"]!["resourceVersion"]);
    }
}
