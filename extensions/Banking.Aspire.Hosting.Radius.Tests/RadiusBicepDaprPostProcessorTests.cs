using Banking.Aspire.Hosting.Radius;
using Xunit;

namespace Banking.Aspire.Hosting.Radius.Tests;

public sealed class RadiusBicepDaprPostProcessorTests
{
    private const string Input = """
        extension radius

        resource accounts 'Radius.Compute/containers@2025-08-01-preview' = {
          name: 'accounts'
          properties: {
            containers: {
              accounts: {
                image: 'localhost:5001/accounts:latest'
              }
            }
            application: app.id
            environment: radiusenv.id
          }
        }
        """;

    [Fact]
    public void ApplyAddsDaprExtensionToMatchingRadiusWorkload()
    {
        var sidecars = new Dictionary<string, RadiusDaprSidecarSettings>
        {
            ["accounts"] = new("accounts", 8085, "grpc", null)
        };

        var result = RadiusBicepDaprPostProcessor.Apply(Input, sidecars);

        Assert.Contains("extensions: {", result);
        Assert.Contains("daprSidecar: {", result);
        Assert.Contains("appId: 'accounts'", result);
        Assert.Contains("appPort: 8085", result);
        Assert.DoesNotContain("appProtocol:", result);
    }

    [Fact]
    public void ApplyIsIdempotent()
    {
        var sidecars = new Dictionary<string, RadiusDaprSidecarSettings>
        {
            ["accounts"] = new("accounts", 8085, "grpc", null)
        };

        var once = RadiusBicepDaprPostProcessor.Apply(Input, sidecars);
        var twice = RadiusBicepDaprPostProcessor.Apply(once, sidecars);

        Assert.Equal(once, twice);
    }

    [Fact]
    public void ApplyFailsWhenRadiusDidNotPublishTheDaprWorkload()
    {
        var sidecars = new Dictionary<string, RadiusDaprSidecarSettings>
        {
            ["missing"] = new("missing", 8080, "http", null)
        };

        var exception = Assert.Throws<InvalidOperationException>(
            () => RadiusBicepDaprPostProcessor.Apply(Input, sidecars));

        Assert.Contains("missing", exception.Message);
    }

    [Fact]
    public void ApplyCorrectsRadiusPreviewPubSubRecipeLocation()
    {
        const string input = """
            templatePath: 'ghcr.io/radius-project/recipes/local-dev/daprpubsubbrokers:latest'

            resource accounts 'Radius.Compute/containers@2025-08-01-preview' = {
              name: 'accounts'
              properties: {
                containers: {}
              }
            }
            """;
        var sidecars = new Dictionary<string, RadiusDaprSidecarSettings>
        {
            ["accounts"] = new("accounts", 8085, "grpc", null)
        };

        var result = RadiusBicepDaprPostProcessor.Apply(input, sidecars);

        Assert.Contains("local-dev/pubsubbrokers:latest", result);
        Assert.DoesNotContain("local-dev/daprpubsubbrokers:latest", result);
    }

    [Fact]
    public void AddImageTagParameterUsesAWorkloadSpecificParameter()
    {
        const string input = """
            extension radius

            resource web 'Radius.Compute/containers@2025-08-01-preview' = {
              name: 'banking-web'
              properties: {
                containers: {
                  web: {
                    image: 'localhost:5001/banking-web:latest'
                  }
                }
              }
            }
            """;

        var result = RadiusReleaseArtifactExtensions.AddImageTagParameter(input, "banking-web");

        Assert.Contains("param banking_web_image_tag string", result);
        Assert.Contains("image: 'localhost:5001/banking-web:${banking_web_image_tag}'", result);
        Assert.DoesNotContain("release_tag", result);
    }
}
