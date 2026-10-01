using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;
using Aspire.Hosting.Radius;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using System.Text.RegularExpressions;

namespace Banking.Aspire.Hosting.Radius;

public static class RadiusReleaseArtifactExtensions
{
    public const string PipelineStepName = "publish-radius-release-artifacts";

#pragma warning disable ASPIREPIPELINES001, ASPIREPIPELINES004
    public static IResourceBuilder<T> AddRadiusReleaseArtifacts<T>(
        this IDistributedApplicationBuilder builder,
        IResourceBuilder<T> radius,
        params IResource[] workloads)
        where T : IResource
    {
        ArgumentNullException.ThrowIfNull(builder);
        ArgumentNullException.ThrowIfNull(radius);
        ArgumentNullException.ThrowIfNull(workloads);

        if (!builder.ExecutionContext.IsPublishMode)
        {
            return radius;
        }

        var workloadNames = workloads
            .Select(resource => resource.Name)
            .Distinct(StringComparer.Ordinal)
            .Order(StringComparer.Ordinal)
            .ToArray();
        var daprSidecars = RadiusDaprPublishingExtensions.GetDaprSidecars(workloads);
        var daprComponentNames = builder.Resources
            .OfType<CommunityToolkit.Aspire.Hosting.Dapr.IDaprComponentResource>()
            .Select(component => component.Name)
            .Distinct(StringComparer.Ordinal)
            .Order(StringComparer.Ordinal)
            .ToArray();
        var kubernetesNamespace = radius.Resource is RadiusEnvironmentResource radiusEnvironment
            ? radiusEnvironment.Namespace
            : throw new InvalidOperationException(
                $"Resource '{radius.Resource.Name}' is not a Radius environment resource.");

        ReplaceRadiusDeploymentStep(
            radius,
            workloadNames,
            daprSidecars,
            daprComponentNames,
            kubernetesNamespace);

        builder.Pipeline.AddStep(
            PipelineStepName,
            async context =>
            {
                var pipelineOutputDirectory = context.Services
                    .GetRequiredService<IPipelineOutputService>()
                    .GetOutputDirectory();
                var sourcePath = Path.Combine(pipelineOutputDirectory, "app.bicep");
                var radiusOutputDirectory = Path.Combine(pipelineOutputDirectory, "radius");
                var servicesDirectory = Path.Combine(radiusOutputDirectory, "services");
                var source = await File.ReadAllTextAsync(sourcePath, context.CancellationToken);
                var artifacts = RadiusReleaseArtifactSplitter.Split(source, workloadNames);

                Directory.CreateDirectory(servicesDirectory);
                await File.WriteAllTextAsync(
                    Path.Combine(radiusOutputDirectory, "infrastructure.bicep"),
                    artifacts.Infrastructure,
                    context.CancellationToken);

                // The built-in Radius deploy entry point must no longer contain workloads. Keeping
                // app.bicep infrastructure-only prevents the monolithic and selective paths from
                // managing the same Radius.Compute/containers resources.
                await File.WriteAllTextAsync(
                    sourcePath,
                    artifacts.Infrastructure,
                    context.CancellationToken);

                foreach (var (name, bicep) in artifacts.Workloads)
                {
                    await File.WriteAllTextAsync(
                        Path.Combine(servicesDirectory, $"{name}.bicep"),
                        AddReleaseTagParameter(bicep, name),
                        context.CancellationToken);
                }

                context.Logger.LogInformation(
                    "Generated Radius infrastructure artifact and {Count} independent workload artifacts in {OutputDirectory}",
                    artifacts.Workloads.Count,
                    radiusOutputDirectory);
            },
            dependsOn: RadiusDaprPublishingExtensions.PipelineStepName,
            requiredBy: $"deploy-radius-{radius.Resource.Name}");

        return radius;
    }

    private static string AddReleaseTagParameter(string bicep, string workloadName)
    {
        var image = new Regex(
            $"(?m)(image:\\s*'[^']*/{Regex.Escape(workloadName)}):latest(')",
            RegexOptions.CultureInvariant);
        var tagged = image.Replace(
            bicep,
            match => $"{match.Groups[1].Value}:${{release_tag}}{match.Groups[2].Value}");
        if (string.Equals(tagged, bicep, StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                $"Could not find the generated image for Radius workload '{workloadName}'.");
        }

        return tagged.Replace(
            "extension radius",
            "extension radius\n\nparam release_tag string",
            StringComparison.Ordinal);
    }

    private static void ReplaceRadiusDeploymentStep<T>(
        IResourceBuilder<T> radius,
        IReadOnlyCollection<string> workloadNames,
        IReadOnlyDictionary<string, RadiusDaprSidecarSettings> daprSidecars,
        IReadOnlyCollection<string> daprComponentNames,
        string kubernetesNamespace)
        where T : IResource
    {
        var deploymentStepName = $"deploy-radius-{radius.Resource.Name}";
        var originalAnnotations = radius.Resource.Annotations
            .OfType<PipelineStepAnnotation>()
            .ToArray();

        if (originalAnnotations.Length == 0)
        {
            throw new InvalidOperationException(
                $"Radius resource '{radius.Resource.Name}' does not expose a deployment pipeline annotation.");
        }

        foreach (var annotation in originalAnnotations)
        {
            radius.Resource.Annotations.Remove(annotation);
        }

        radius.Resource.Annotations.Add(new PipelineStepAnnotation(async factoryContext =>
        {
            var steps = new List<PipelineStep>();
            foreach (var annotation in originalAnnotations)
            {
                steps.AddRange(await annotation.CreateStepsAsync(factoryContext));
            }

            var removed = steps.RemoveAll(step =>
                string.Equals(step.Name, deploymentStepName, StringComparison.Ordinal));
            if (removed != 1)
            {
                throw new InvalidOperationException(
                    $"Expected one Radius deployment step named '{deploymentStepName}', but found {removed}.");
            }

            steps.Add(CreateDeploymentStep(
                deploymentStepName,
                "Deploy Radius infrastructure",
                Path.Combine("radius", "infrastructure.bicep"),
                radius.Resource,
                [PipelineStepName, WellKnownPipelineSteps.DeployPrereq],
                [WellKnownPipelineSteps.Deploy],
                kubernetesNamespace: kubernetesNamespace,
                daprComponentNames: daprComponentNames));

            foreach (var workloadName in workloadNames)
            {
                daprSidecars.TryGetValue(workloadName, out var daprSidecar);
                steps.Add(CreateDeploymentStep(
                    $"deploy-radius-{workloadName}",
                    $"Deploy Radius workload '{workloadName}'",
                    Path.Combine("radius", "services", $"{workloadName}.bicep"),
                    radius.Resource,
                    [deploymentStepName, $"push-{workloadName}"],
                    [WellKnownPipelineSteps.Deploy],
                    workloadName,
                    kubernetesNamespace,
                    daprSidecar));
            }

            return steps;
        }));
    }

    private static PipelineStep CreateDeploymentStep(
        string name,
        string description,
        string artifactPath,
        IResource resource,
        string[] dependsOn,
        string[] requiredBy,
        string? workloadName = null,
        string? kubernetesNamespace = null,
        RadiusDaprSidecarSettings? daprSidecar = null,
        IReadOnlyCollection<string>? daprComponentNames = null) => new()
        {
            Name = name,
            Description = description,
            Action = async context =>
            {
                await RadiusArtifactDeployer.DeployAsync(artifactPath, context);
                if (workloadName is null && kubernetesNamespace is not null && daprComponentNames is not null)
                {
                    foreach (var componentName in daprComponentNames)
                    {
                        await KubernetesDaprDeploymentPatcher.MirrorComponentAsync(
                            componentName,
                            kubernetesNamespace,
                            context);
                    }
                }
                if (workloadName is not null && kubernetesNamespace is not null && daprSidecar is not null)
                {
                    await KubernetesDaprDeploymentPatcher.PatchAsync(
                        workloadName,
                        kubernetesNamespace,
                        daprSidecar.AppProtocol,
                        context);
                }
            },
            DependsOnSteps = [.. dependsOn],
            RequiredBySteps = [.. requiredBy],
            Resource = resource
        };
#pragma warning restore ASPIREPIPELINES001, ASPIREPIPELINES004
}
