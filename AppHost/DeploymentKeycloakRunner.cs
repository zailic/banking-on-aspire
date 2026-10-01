using System.Net;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json.Nodes;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.Logging;

internal static class DeploymentKeycloakRunner
{
#pragma warning disable ASPIREPIPELINES001
    public static async Task RunAsync(
        string kubernetesNamespace,
        KeycloakResource keycloak,
        ParameterResource bankingWebClientSecret,
        string realm,
        string realmFile,
        PipelineStepContext context)
    {
        if (!File.Exists(realmFile))
        {
            throw new FileNotFoundException("The Keycloak realm file was not found.", realmFile);
        }

        var username = keycloak.AdminUserNameParameter is { } usernameParameter
            ? await usernameParameter.GetValueAsync(context.CancellationToken)
            : "admin";
        var password = await keycloak.AdminPasswordParameter.GetValueAsync(context.CancellationToken);
        var clientSecret = await bankingWebClientSecret.GetValueAsync(context.CancellationToken);
        if (string.IsNullOrWhiteSpace(username) ||
            string.IsNullOrWhiteSpace(password) ||
            string.IsNullOrWhiteSpace(clientSecret))
        {
            throw new InvalidOperationException("The Keycloak deployment credentials are not configured.");
        }

        var realmJson = JsonNode.Parse(
                await File.ReadAllTextAsync(realmFile, context.CancellationToken)) as JsonObject
            ?? throw new InvalidOperationException("The Keycloak realm file must contain a JSON object.");
        realmJson["realm"] = realm;
        ConfigureBankingWebClient(realmJson, clientSecret, "http://localhost:7443");

        // Radius reports the resource deployment before Kubernetes necessarily replaces the
        // previous Keycloak pod. Importing through the Service before this barrier can update
        // the old pod and leave the new pod without the realm.
        await ProcessRunner.RunAsync(
            "kubectl",
            [
                "rollout", "status", "deployment/keycloak",
                "--namespace", kubernetesNamespace,
                "--timeout", "2m"
            ],
            Path.GetFullPath("."),
            new Dictionary<string, string>(),
            context.Logger,
            context.CancellationToken);

        await using var portForward = await StartPortForwardAsync(
            kubernetesNamespace,
            context.Logger,
            context.CancellationToken);
        using var client = await CreateAdminClientAsync(
            portForward.LocalPort,
            username,
            password,
            context.Logger,
            context.CancellationToken);

        var encodedRealm = Uri.EscapeDataString(realm);
        using var lookupResponse = await client.GetAsync(
            $"admin/realms/{encodedRealm}",
            context.CancellationToken);

        if (lookupResponse.StatusCode == HttpStatusCode.NotFound)
        {
            using var createContent = CreateJsonContent(realmJson);
            using var createResponse = await client.PostAsync(
                "admin/realms",
                createContent,
                context.CancellationToken);
            if (createResponse.StatusCode != HttpStatusCode.Conflict)
            {
                await EnsureSuccessAsync(createResponse, "create", realm, context.CancellationToken);
                context.Logger.LogInformation(
                    "Created Keycloak realm '{Realm}' from {RealmFile}",
                    realm,
                    realmFile);
                return;
            }

            context.Logger.LogInformation(
                "Keycloak realm '{Realm}' was created concurrently; applying the idempotent import",
                realm);
        }
        else
        {
            await EnsureSuccessAsync(lookupResponse, "look up", realm, context.CancellationToken);
        }

        realmJson["ifResourceExists"] = "OVERWRITE";
        using var importContent = CreateJsonContent(realmJson);
        using var importResponse = await client.PostAsync(
            $"admin/realms/{encodedRealm}/partialImport",
            importContent,
            context.CancellationToken);
        await EnsureSuccessAsync(importResponse, "import", realm, context.CancellationToken);
        context.Logger.LogInformation(
            "Updated Keycloak realm '{Realm}' from {RealmFile}",
            realm,
            realmFile);
    }
#pragma warning restore ASPIREPIPELINES001

    private static async Task<KubernetesPortForward> StartPortForwardAsync(
        string kubernetesNamespace,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        var deadline = DateTimeOffset.UtcNow.AddMinutes(2);
        Exception? lastException = null;
        while (DateTimeOffset.UtcNow < deadline)
        {
            try
            {
                return await KubernetesPortForward.StartAsync(
                    kubernetesNamespace,
                    "keycloak-keycloak",
                    8080,
                    logger,
                    cancellationToken);
            }
            catch (Exception exception) when (exception is not OperationCanceledException)
            {
                lastException = exception;
                logger.LogInformation("Waiting for the Radius Keycloak service to become available");
                await Task.Delay(TimeSpan.FromSeconds(2), cancellationToken);
            }
        }

        throw new TimeoutException(
            "The Radius Keycloak service did not become available within 2 minutes.",
            lastException);
    }

    private static async Task<HttpClient> CreateAdminClientAsync(
        int localPort,
        string username,
        string password,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        var client = new HttpClient
        {
            BaseAddress = new Uri($"http://127.0.0.1:{localPort}/"),
            Timeout = TimeSpan.FromSeconds(10)
        };
        var deadline = DateTimeOffset.UtcNow.AddMinutes(2);
        Exception? lastException = null;

        while (DateTimeOffset.UtcNow < deadline)
        {
            try
            {
                using var tokenContent = new FormUrlEncodedContent(new Dictionary<string, string>
                {
                    ["client_id"] = "admin-cli",
                    ["grant_type"] = "password",
                    ["username"] = username,
                    ["password"] = password
                });
                using var response = await client.PostAsync(
                    "realms/master/protocol/openid-connect/token",
                    tokenContent,
                    cancellationToken);
                if (response.IsSuccessStatusCode)
                {
                    var tokenJson = JsonNode.Parse(
                        await response.Content.ReadAsStringAsync(cancellationToken));
                    var accessToken = tokenJson?["access_token"]?.GetValue<string>()
                        ?? throw new InvalidOperationException("Keycloak did not return an access token.");
                    client.DefaultRequestHeaders.Authorization =
                        new AuthenticationHeaderValue("Bearer", accessToken);
                    return client;
                }

                lastException = new InvalidOperationException(
                    $"Keycloak admin authentication returned HTTP {(int)response.StatusCode}.");
            }
            catch (Exception exception) when (exception is not OperationCanceledException)
            {
                lastException = exception;
            }

            logger.LogInformation("Waiting for Keycloak admin API to become ready");
            await Task.Delay(TimeSpan.FromSeconds(2), cancellationToken);
        }

        client.Dispose();
        throw new TimeoutException(
            "The Keycloak admin API did not become ready within 2 minutes.",
            lastException);
    }

    private static StringContent CreateJsonContent(JsonObject json) =>
        new(json.ToJsonString(), Encoding.UTF8, "application/json");

    private static void ConfigureBankingWebClient(
        JsonObject realmJson,
        string clientSecret,
        string publicBaseUrl)
    {
        var client = realmJson["clients"]?.AsArray()
            .OfType<JsonObject>()
            .SingleOrDefault(candidate =>
                string.Equals(
                    candidate["clientId"]?.GetValue<string>(),
                    "banking-on-aspire-app",
                    StringComparison.Ordinal))
            ?? throw new InvalidOperationException(
                "The Keycloak realm does not contain the 'banking-on-aspire-app' client.");

        publicBaseUrl = publicBaseUrl.TrimEnd('/');
        client["secret"] = clientSecret;
        client["rootUrl"] = publicBaseUrl;
        client["adminUrl"] = publicBaseUrl;
        client["redirectUris"] = new JsonArray(
            $"{publicBaseUrl}/signin-oidc",
            $"{publicBaseUrl}/signout-callback-oidc");
        client["webOrigins"] = new JsonArray(publicBaseUrl);
    }

    private static async Task EnsureSuccessAsync(
        HttpResponseMessage response,
        string operation,
        string realm,
        CancellationToken cancellationToken)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var responseBody = await response.Content.ReadAsStringAsync(cancellationToken);
        throw new InvalidOperationException(
            $"Failed to {operation} Keycloak realm '{realm}' (HTTP {(int)response.StatusCode}): {responseBody}");
    }
}
