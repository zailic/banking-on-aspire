using System.Security.Claims;
using System.Text.Json;
using Microsoft.AspNetCore.WebUtilities;

namespace Banking.Web.Auth;

internal static class KeycloakClientRoles
{
    public static void AddTo(ClaimsIdentity identity, string accessToken, string clientId)
    {
        var segments = accessToken.Split('.');
        if (segments.Length < 2)
        {
            return;
        }

        try
        {
            using var payload = JsonDocument.Parse(WebEncoders.Base64UrlDecode(segments[1]));
            if (!payload.RootElement.TryGetProperty("resource_access", out var resourceAccess)
                || !resourceAccess.TryGetProperty(clientId, out var client)
                || !client.TryGetProperty("roles", out var roles)
                || roles.ValueKind != JsonValueKind.Array)
            {
                return;
            }

            foreach (var role in roles.EnumerateArray())
            {
                var value = role.GetString();
                if (!string.IsNullOrWhiteSpace(value)
                    && !identity.HasClaim(ClaimTypes.Role, value))
                {
                    identity.AddClaim(new Claim(ClaimTypes.Role, value));
                }
            }
        }
        catch (FormatException)
        {
            // Authentication already validates the token. Missing UI roles fail closed.
        }
        catch (JsonException)
        {
            // Authentication already validates the token. Missing UI roles fail closed.
        }
    }
}
