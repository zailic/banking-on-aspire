using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using Aspire.Hosting.ApplicationModel;

internal static class KeycloakCommands
{
    public static IResourceBuilder<KeycloakResource> WithRealmCommands(
        this IResourceBuilder<KeycloakResource> keycloak,
        string realm,
        string realmFile)
    {
        return keycloak
            .WithCommand(
                "export-realm",
                "Export realm",
                context => ExportRealmAsync(keycloak, realm, realmFile, context.CancellationToken),
                new CommandOptions { Description = $"Export '{realm}' to {realmFile}." })
            .WithCommand(
                "import-realm",
                "Import realm",
                context => ImportRealmAsync(keycloak, realm, realmFile, context.CancellationToken),
                new CommandOptions
                {
                    Description = $"Import {realmFile} into the running '{realm}' realm.",
                    ConfirmationMessage = $"Existing resources in '{realm}' may be overwritten. Continue?"
                });
    }

    private static async Task<ExecuteCommandResult> ExportRealmAsync(
        IResourceBuilder<KeycloakResource> keycloak,
        string realm,
        string realmFile,
        CancellationToken cancellationToken)
    {
        try
        {
            using var client = await CreateAdminClientAsync(keycloak, cancellationToken);
            var encodedRealm = Uri.EscapeDataString(realm);
            using var response = await client.PostAsync(
                $"admin/realms/{encodedRealm}/partial-export?exportClients=true&exportGroupsAndRoles=true",
                content: null,
                cancellationToken);

            var responseBody = await response.Content.ReadAsStringAsync(cancellationToken);
            if (!response.IsSuccessStatusCode)
            {
                return CommandResults.Failure(
                    $"Keycloak export failed ({(int)response.StatusCode}): {responseBody}");
            }

            var realmJson = JsonNode.Parse(responseBody)
                ?? throw new InvalidOperationException("Keycloak returned an empty export.");

            Directory.CreateDirectory(Path.GetDirectoryName(realmFile)!);
            var temporaryFile = realmFile + ".tmp";
            await File.WriteAllTextAsync(
                temporaryFile,
                realmJson.ToJsonString(new JsonSerializerOptions { WriteIndented = true }) + Environment.NewLine,
                Encoding.UTF8,
                cancellationToken);
            File.Move(temporaryFile, realmFile, overwrite: true);

            return CommandResults.Success($"Realm exported to {realmFile}.");
        }
        catch (Exception exception) when (exception is not OperationCanceledException)
        {
            return CommandResults.Failure(exception);
        }
    }

    private static async Task<ExecuteCommandResult> ImportRealmAsync(
        IResourceBuilder<KeycloakResource> keycloak,
        string realm,
        string realmFile,
        CancellationToken cancellationToken)
    {
        try
        {
            if (!File.Exists(realmFile))
            {
                return CommandResults.Failure(
                    $"Realm file does not exist: {realmFile}. Export the realm first.");
            }

            var realmJson = JsonNode.Parse(await File.ReadAllTextAsync(realmFile, cancellationToken))
                as JsonObject
                ?? throw new InvalidOperationException("The realm file must contain a JSON object.");
            realmJson["ifResourceExists"] = "OVERWRITE";

            using var client = await CreateAdminClientAsync(keycloak, cancellationToken);
            var encodedRealm = Uri.EscapeDataString(realm);
            using var content = new StringContent(realmJson.ToJsonString(), Encoding.UTF8, "application/json");
            using var response = await client.PostAsync(
                $"admin/realms/{encodedRealm}/partialImport",
                content,
                cancellationToken);

            var responseBody = await response.Content.ReadAsStringAsync(cancellationToken);
            if (!response.IsSuccessStatusCode)
            {
                return CommandResults.Failure(
                    $"Keycloak import failed ({(int)response.StatusCode}): {responseBody}");
            }

            return CommandResults.Success(
                string.IsNullOrWhiteSpace(responseBody)
                    ? $"Realm imported from {realmFile}."
                    : responseBody);
        }
        catch (Exception exception) when (exception is not OperationCanceledException)
        {
            return CommandResults.Failure(exception);
        }
    }

    private static async Task<HttpClient> CreateAdminClientAsync(
        IResourceBuilder<KeycloakResource> keycloak,
        CancellationToken cancellationToken)
    {
        var baseAddress = await keycloak.GetEndpoint("http").GetValueAsync(cancellationToken)
            ?? throw new InvalidOperationException("The Keycloak HTTP endpoint is not available.");
        var username = keycloak.Resource.AdminUserNameParameter is { } usernameParameter
            ? await usernameParameter.GetValueAsync(cancellationToken)
            : "admin";
        if (string.IsNullOrWhiteSpace(username))
        {
            throw new InvalidOperationException("The Keycloak admin username is not available.");
        }

        var password = await keycloak.Resource.AdminPasswordParameter.GetValueAsync(cancellationToken)
            ?? throw new InvalidOperationException("The Keycloak admin password is not available.");

        var client = new HttpClient { BaseAddress = new Uri(baseAddress.TrimEnd('/') + "/") };
        try
        {
            using var tokenContent = new FormUrlEncodedContent(new Dictionary<string, string>
            {
                ["client_id"] = "admin-cli",
                ["grant_type"] = "password",
                ["username"] = username,
                ["password"] = password
            });
            using var tokenResponse = await client.PostAsync(
                "realms/master/protocol/openid-connect/token",
                tokenContent,
                cancellationToken);
            var tokenResponseBody = await tokenResponse.Content.ReadAsStringAsync(cancellationToken);

            if (!tokenResponse.IsSuccessStatusCode)
            {
                throw new InvalidOperationException(
                    $"Keycloak admin authentication failed ({(int)tokenResponse.StatusCode}): {tokenResponseBody}");
            }

            using var tokenJson = JsonDocument.Parse(tokenResponseBody);
            var accessToken = tokenJson.RootElement.GetProperty("access_token").GetString()
                ?? throw new InvalidOperationException("Keycloak did not return an access token.");
            client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", accessToken);
            return client;
        }
        catch
        {
            client.Dispose();
            throw;
        }
    }
}
