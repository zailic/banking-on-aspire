using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Radius;
using CommunityToolkit.Aspire.Hosting.Dapr;

namespace Banking.Aspire.Hosting.Radius;

/// <summary>
/// Compatibility resources for the preview Radius publisher.
/// </summary>
public static class RadiusDaprComponentExtensions
{
    /// <summary>
    /// Adds a generic Dapr Pub/Sub component using the CLR resource name currently
    /// recognized by Aspire.Hosting.Radius.
    /// </summary>
    public static IResourceBuilder<IDaprComponentResource> AddRadiusDaprPubSub(
        this IDistributedApplicationBuilder builder,
        string name,
        IResourceBuilder<RadiusEnvironmentResource> radius)
    {
        ArgumentNullException.ThrowIfNull(builder);
        ArgumentNullException.ThrowIfNull(radius);

        // Radius 13.5 preview maps Dapr resources by their simple CLR type name.
        // CommunityToolkit 13.2 represents every component as DaprComponentResource,
        // while Radius still expects DaprPubSubResource. Keep this shim isolated so it
        // can be removed when the upstream packages agree on the app-model type.
        var resource = new DaprPubSubResource(name);
        builder
            .AddResource(resource)
            .WithComputeEnvironment(radius);

        // Radius' preview PrepareDeploymentTargets step only enumerates containers,
        // emulators, and projects. Backing Dapr resources therefore need the target
        // materialized here even though they implement IComputeResource.
        resource.Annotations.Add(new DeploymentTargetAnnotation(radius.Resource)
        {
            ComputeEnvironment = radius.Resource
        });

        return builder.CreateResourceBuilder<IDaprComponentResource>(resource);
    }
}

/// <remarks>
/// The simple type name is part of the Radius 13.5 preview mapping contract.
/// </remarks>
public sealed class DaprPubSubResource(string name) :
    Resource(name),
    IDaprComponentResource,
    IComputeResource
{
    public string Type => "pubsub";

    public DaprComponentOptions? Options => null;
}
