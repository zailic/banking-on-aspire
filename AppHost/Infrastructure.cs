using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Radius;
using Banking.Aspire.Hosting.Radius;
using CommunityToolkit.Aspire.Hosting.Dapr;

internal sealed record BankingInfrastructure(
    IResourceBuilder<RadiusEnvironmentResource> Radius,
    IResourceBuilder<ContainerRegistryResource> LocalRegistry,
    BankingReleaseVersions ReleaseVersions,
    IResourceBuilder<ParameterResource> BankingWebClientSecret,
    IResourceBuilder<ParameterResource> PostgresPassword,
    IResourceBuilder<KeycloakResource> Keycloak,
    IResourceBuilder<PostgresDatabaseResource> UsersDb,
    IResourceBuilder<PostgresDatabaseResource> TransactionsDb,
    IResourceBuilder<RedisResource> Redis,
    IResourceBuilder<IDaprComponentResource> PubSub);

internal static class Infrastructure
{
    public static BankingInfrastructure AddBankingInfrastructure(
        this IDistributedApplicationBuilder builder)
    {
        var radius = builder.AddRadiusEnvironment("radius");
        var registryEndpoint = builder.AddParameter(
            "registry-endpoint", "localhost:5001", publishValueAsDefault: true);
        var registryRepository = builder.AddParameter(
            "registry-repository", "banking-on-aspire", publishValueAsDefault: true);
        var localRegistry = builder.AddContainerRegistry(
            "local-registry", registryEndpoint, registryRepository);
        var releaseVersions = BankingReleaseVersions.FromEnvironment();
        builder.AddParameter("accounts-image-tag", releaseVersions.Accounts, publishValueAsDefault: true);
        builder.AddParameter("transactions-image-tag", releaseVersions.Transactions, publishValueAsDefault: true);
        builder.AddParameter("contacts-image-tag", releaseVersions.Contacts, publishValueAsDefault: true);
        builder.AddParameter("users-image-tag", releaseVersions.Users, publishValueAsDefault: true);
        builder.AddParameter("banking-web-image-tag", releaseVersions.BankingWeb, publishValueAsDefault: true);

        if (builder.ExecutionContext.IsPublishMode &&
            string.IsNullOrWhiteSpace(builder.Configuration["Parameters:postgres-password"]))
        {
            throw new InvalidOperationException(
                "A stable PostgreSQL deployment password is required. Configure " +
                "'Parameters:postgres-password' with dotnet user-secrets locally or " +
                "'Parameters__postgres-password' in the deployment environment.");
        }

        var bankingWebClientSecret = builder.AddParameter("banking-web-client-secret", secret: true);
        var postgresPassword = builder.AddParameter("postgres-password", secret: true);
        var keycloakPassword = builder.AddParameter("keycloak-password", secret: true);

        var realmDirectory = Path.GetFullPath("dev/keycloak");
        const string developmentRealm = "banking-on-aspire";
        var realmFile = Path.Combine(realmDirectory, $"{developmentRealm}-realm.json");

        var keycloak = builder.AddKeycloak(
                "keycloak",
                port: 8081,
                adminPassword: keycloakPassword)
            // Local services share the localhost cookie namespace even when their ports differ.
            .WithEnvironment("QUARKUS_HTTP_LIMITS_MAX_HEADER_SIZE", "64K")
            .WithEnvironment("QUARKUS_HTTP_LIMITS_MAX_HEADER_LIST_SIZE", "65536")
            .WithDataVolume("boa-keycloak-data")
            .WithRealmImport(realmDirectory)
            .WithRealmCommands(developmentRealm, realmFile);

        if (builder.ExecutionContext.IsPublishMode)
        {
            keycloak
                .WithEnvironment("KC_HOSTNAME", "http://localhost:8081")
                .WithEnvironment("KC_HOSTNAME_BACKCHANNEL_DYNAMIC", "true");
        }

        var postgres = builder.AddPostgres("postgres", password: postgresPassword)
            .WithDataVolume("boa-postgres-data");
        var usersDb = postgres.AddDatabase("usersdb");
        var transactionsDb = postgres.AddDatabase("transactionsdb");
        var redis = builder.AddRedis("redis");
        var pubSub = builder.ExecutionContext.IsPublishMode
            ? builder.AddRadiusDaprPubSub("pubsub", radius)
            : builder.AddDaprPubSub("pubsub")
                .WithMetadata("redisHost", redis.GetEndpoint("tcp"));

        builder.ConfigureDeploymentPipeline(radius);
        builder.AddRadiusDaprPublishing(radius, dependsOn: "radius-schema-compatibility");
        AddPostgresPersistenceStep(builder, radius);
        AddKeycloakPersistenceStep(builder, radius);
        AddKeycloakDeploymentStep(
            builder,
            radius,
            keycloak,
            bankingWebClientSecret,
            developmentRealm,
            realmFile);

        return new BankingInfrastructure(
            radius,
            localRegistry,
            releaseVersions,
            bankingWebClientSecret,
            postgresPassword,
            keycloak,
            usersDb,
            transactionsDb,
            redis,
            pubSub);
    }

#pragma warning disable ASPIREPIPELINES001
    private static void AddPostgresPersistenceStep(
        IDistributedApplicationBuilder builder,
        IResourceBuilder<RadiusEnvironmentResource> radius)
    {
        if (!builder.ExecutionContext.IsPublishMode)
        {
            return;
        }

        builder.Pipeline.AddStep(
            "ensure-radius-postgres-persistence",
            context => DeploymentPostgresPersistenceRunner.RunAsync(
                radius.Resource.Namespace,
                context),
            dependsOn: $"deploy-radius-{radius.Resource.Name}",
            requiredBy: "configure-radius-keycloak");
    }
#pragma warning restore ASPIREPIPELINES001

#pragma warning disable ASPIREPIPELINES001
    private static void AddKeycloakPersistenceStep(
        IDistributedApplicationBuilder builder,
        IResourceBuilder<RadiusEnvironmentResource> radius)
    {
        if (!builder.ExecutionContext.IsPublishMode)
        {
            return;
        }

        builder.Pipeline.AddStep(
            "ensure-radius-keycloak-persistence",
            context => DeploymentPostgresPersistenceRunner.RunKeycloakAsync(
                radius.Resource.Namespace,
                context),
            dependsOn: "ensure-radius-postgres-persistence",
            requiredBy: "configure-radius-keycloak");
    }
#pragma warning restore ASPIREPIPELINES001

#pragma warning disable ASPIREPIPELINES001
    private static void AddKeycloakDeploymentStep(
        IDistributedApplicationBuilder builder,
        IResourceBuilder<RadiusEnvironmentResource> radius,
        IResourceBuilder<KeycloakResource> keycloak,
        IResourceBuilder<ParameterResource> bankingWebClientSecret,
        string realm,
        string realmFile)
    {
        if (!builder.ExecutionContext.IsPublishMode)
        {
            return;
        }

        builder.Pipeline.AddStep(
            "configure-radius-keycloak",
            context => DeploymentKeycloakRunner.RunAsync(
                radius.Resource.Namespace,
                keycloak.Resource,
                bankingWebClientSecret.Resource,
                realm,
                realmFile,
                context),
            dependsOn: "ensure-radius-keycloak-persistence",
            requiredBy: "deploy-radius-banking-web");
    }
#pragma warning restore ASPIREPIPELINES001
}
