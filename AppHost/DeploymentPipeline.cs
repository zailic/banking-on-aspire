using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

internal static class DeploymentPipeline
{
#pragma warning disable ASPIREPIPELINES001, ASPIREPIPELINES004
    public static void ConfigureDeploymentPipeline<T>(
        this IDistributedApplicationBuilder builder,
        IResourceBuilder<T> radius)
        where T : IResource
    {
        if (builder.ExecutionContext.IsPublishMode)
        {
            AddRadiusCompatibilityStep(builder);
        }
    }

    private static void AddRadiusCompatibilityStep(IDistributedApplicationBuilder builder)
    {
        builder.Pipeline.AddStep(
            "radius-schema-compatibility",
            async context =>
            {
                var outputDirectory = context.Services
                    .GetRequiredService<IPipelineOutputService>()
                    .GetOutputDirectory();
                var bicepPath = Path.Combine(outputDirectory, "app.bicep");
                var bicepConfigPath = Path.Combine(outputDirectory, "bicepconfig.json");
                var bicep = await File.ReadAllTextAsync(bicepPath, context.CancellationToken);
                var bicepConfig = await File.ReadAllTextAsync(bicepConfigPath, context.CancellationToken);
                var compatibleBicep = bicep
                    .Replace("recipeKind:", "kind:", StringComparison.Ordinal)
                    .Replace("recipeLocation:", "source:", StringComparison.Ordinal)
                    .Replace(
                        "ghcr.io/radius-project/recipes/local-dev/postgresqldatabases:latest",
                        "ghcr.io/radius-project/recipes/kubernetes/postgresql:0.53.0",
                        StringComparison.Ordinal)
                    .Replace(
                        "resource postgres 'Radius.Data/postgreSqlDatabases@2025-08-01-preview' = {\n  name: 'postgres'\n  properties: {",
                        "resource postgres 'Radius.Data/postgreSqlDatabases@2025-08-01-preview' = {\n  name: 'postgres'\n  properties: {\n    username: 'postgres'\n    password: postgres_password",
                        StringComparison.Ordinal)
                    .Replace(
                        "        image: 'quay.io/keycloak/keycloak:26.6'\n        env: {",
                        "        image: 'quay.io/keycloak/keycloak:26.6'\n        args: [\n          'start-dev'\n        ]\n        env: {",
                        StringComparison.Ordinal);
                var compatibleBicepConfig = bicepConfig.Replace(
                    "br:biceptypes.azurecr.io/radius:0.59",
                    "br:biceptypes.azurecr.io/radius:0.60",
                    StringComparison.Ordinal);

                EnsureRadiusOutputIsCompatible(compatibleBicep, compatibleBicepConfig);

                await File.WriteAllTextAsync(bicepPath, compatibleBicep, context.CancellationToken);
                await File.WriteAllTextAsync(bicepConfigPath, compatibleBicepConfig, context.CancellationToken);
                context.Logger.LogInformation("Applied Radius 0.60 recipe schema compatibility adjustments");
            },
            dependsOn: "publish-radius-radius",
            requiredBy: "deploy-radius-radius");
    }

    private static void EnsureRadiusOutputIsCompatible(string bicep, string bicepConfig)
    {
        if (!bicep.Contains(
                "source: 'ghcr.io/radius-project/recipes/kubernetes/postgresql:0.53.0'",
                StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                "The generated Radius Bicep does not contain the expected Kubernetes PostgreSQL recipe.");
        }

        if (!bicep.Contains("password: postgres_password", StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                "The generated Radius Bicep does not contain the required PostgreSQL credentials.");
        }

        if (!bicep.Contains("'start-dev'", StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                "The generated Radius Bicep does not contain the required Keycloak startup command.");
        }

        if (!bicepConfig.Contains("br:biceptypes.azurecr.io/radius:0.60", StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                "The generated Bicep configuration does not reference the Radius 0.60 extension.");
        }
    }
#pragma warning restore ASPIREPIPELINES001, ASPIREPIPELINES004
}
