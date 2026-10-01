using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;
using CommunityToolkit.Aspire.Hosting.Dapr;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace Banking.Aspire.Hosting.Radius;

public static class RadiusDaprPublishingExtensions
{
    public const string PipelineStepName = "radius-dapr-annotations";

#pragma warning disable ASPIREPIPELINES001, ASPIREPIPELINES004
    public static IResourceBuilder<T> AddRadiusDaprPublishing<T>(
        this IDistributedApplicationBuilder builder,
        IResourceBuilder<T> radius,
        string? dependsOn = null)
        where T : IResource
    {
        ArgumentNullException.ThrowIfNull(builder);
        ArgumentNullException.ThrowIfNull(radius);

        if (!builder.ExecutionContext.IsPublishMode)
        {
            return radius;
        }

        var publishStep = $"publish-radius-{radius.Resource.Name}";
        var deployStep = $"deploy-radius-{radius.Resource.Name}";

        builder.Pipeline.AddStep(
            PipelineStepName,
            async context =>
            {
                foreach (var component in context.Model.Resources.OfType<IDaprComponentResource>())
                {
                    context.Logger.LogDebug(
                        "Radius Dapr component candidate '{Name}' uses model type '{Type}' and annotations: {Annotations}",
                        component.Name,
                        component.GetType().Name,
                        string.Join(", ", component.Annotations.Select(annotation => annotation.GetType().Name)));
                }

                var sidecars = GetDaprSidecars(builder.Resources);
                if (sidecars.Count == 0)
                {
                    context.Logger.LogInformation("No Dapr-enabled Radius workloads were found");
                    return;
                }

                var unsupportedProtocols = sidecars
                    .Where(pair => !string.IsNullOrWhiteSpace(pair.Value.AppProtocol) &&
                                   !string.Equals(pair.Value.AppProtocol, "http", StringComparison.OrdinalIgnoreCase))
                    .Select(pair => $"{pair.Key} ({pair.Value.AppProtocol})")
                    .ToArray();
                if (unsupportedProtocols.Length > 0)
                {
                    context.Logger.LogWarning(
                        "Radius 0.60 cannot publish the Dapr app protocol for: {Resources}. " +
                        "The selective Radius deployment step will apply the corresponding " +
                        "dapr.io/app-protocol annotation directly to the Kubernetes Deployment.",
                        string.Join(", ", unsupportedProtocols));
                }

                var outputDirectory = context.Services
                    .GetRequiredService<IPipelineOutputService>()
                    .GetOutputDirectory();
                var bicepPath = Path.Combine(outputDirectory, "app.bicep");
                var bicep = await File.ReadAllTextAsync(bicepPath, context.CancellationToken);
                var updatedBicep = RadiusBicepDaprPostProcessor.Apply(bicep, sidecars);

                await File.WriteAllTextAsync(bicepPath, updatedBicep, context.CancellationToken);
                context.Logger.LogInformation(
                    "Added Dapr sidecar configuration to {Count} Radius workloads",
                    sidecars.Count);
            },
            dependsOn: dependsOn ?? publishStep,
            requiredBy: deployStep);

        return radius;
    }
#pragma warning restore ASPIREPIPELINES001, ASPIREPIPELINES004

    internal static IReadOnlyDictionary<string, RadiusDaprSidecarSettings> GetDaprSidecars(
        IEnumerable<IResource> resources)
    {
        var result = new Dictionary<string, RadiusDaprSidecarSettings>(StringComparer.Ordinal);

        foreach (var resource in resources)
        {
            if (!resource.TryGetLastAnnotation<DaprSidecarAnnotation>(out var sidecarAnnotation))
            {
                continue;
            }

            var options = sidecarAnnotation.Sidecar.Annotations
                .OfType<DaprSidecarOptionsAnnotation>()
                .LastOrDefault()
                ?.Options;

            result.Add(
                resource.Name,
                new RadiusDaprSidecarSettings(
                    options?.AppId ?? resource.Name,
                    options?.AppPort,
                    options?.AppProtocol,
                    options?.Config));
        }

        return result;
    }
}

public sealed record RadiusDaprSidecarSettings(
    string AppId,
    int? AppPort,
    string? AppProtocol,
    string? Config);
