using System.Globalization;
using System.Net.Http.Json;
using System.Text.Json.Serialization;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;

namespace Banking.Web.Auth;

public sealed class AccessTokenProvider(
    IHttpContextAccessor httpContextAccessor,
    IHttpClientFactory httpClientFactory,
    IConfiguration configuration)
{
    private static readonly TimeSpan RefreshSkew = TimeSpan.FromSeconds(60);
    private readonly SemaphoreSlim refreshLock = new(1, 1);
    private bool initialized;
    private string? accessToken;
    private string? refreshToken;
    private DateTimeOffset accessTokenExpiresAt;

    public async Task<string> GetAccessTokenAsync(
        bool forceRefresh = false,
        CancellationToken cancellationToken = default)
    {
        await refreshLock.WaitAsync(cancellationToken);
        try
        {
            if (!initialized)
            {
                await InitializeFromAuthenticationTicketAsync();
            }

            if (!forceRefresh
                && !string.IsNullOrWhiteSpace(accessToken)
                && accessTokenExpiresAt > DateTimeOffset.UtcNow.Add(RefreshSkew))
            {
                return accessToken;
            }

            return await RefreshAccessTokenAsync(cancellationToken);
        }
        finally
        {
            refreshLock.Release();
        }
    }

    private async Task InitializeFromAuthenticationTicketAsync()
    {
        var context = httpContextAccessor.HttpContext
            ?? throw new ReauthenticationRequiredException("The authentication session is unavailable.");
        var result = await context.AuthenticateAsync(CookieAuthenticationDefaults.AuthenticationScheme);
        if (!result.Succeeded || result.Properties is null)
        {
            throw new ReauthenticationRequiredException("The authentication session is no longer valid.");
        }

        accessToken = result.Properties.GetTokenValue("access_token");
        refreshToken = result.Properties.GetTokenValue("refresh_token");
        var expiresAt = result.Properties.GetTokenValue("expires_at");
        if (!DateTimeOffset.TryParse(
                expiresAt,
                CultureInfo.InvariantCulture,
                DateTimeStyles.RoundtripKind,
                out accessTokenExpiresAt))
        {
            accessTokenExpiresAt = DateTimeOffset.MinValue;
        }
        initialized = true;
    }

    private async Task<string> RefreshAccessTokenAsync(CancellationToken cancellationToken)
    {
        if (string.IsNullOrWhiteSpace(refreshToken))
        {
            throw new ReauthenticationRequiredException("The session cannot be renewed.");
        }

        var keycloakBaseUrl = configuration["Authentication:KeycloakBaseUrl"]
            ?? throw new InvalidOperationException("Authentication:KeycloakBaseUrl is required.");
        var clientSecret = configuration["Authentication:ClientSecret"]
            ?? throw new InvalidOperationException("Authentication:ClientSecret is required.");
        var tokenEndpoint = $"{keycloakBaseUrl.TrimEnd('/')}/realms/banking-on-aspire/protocol/openid-connect/token";

        using var request = new HttpRequestMessage(HttpMethod.Post, tokenEndpoint)
        {
            Content = new FormUrlEncodedContent(new Dictionary<string, string>
            {
                ["grant_type"] = "refresh_token",
                ["client_id"] = "banking-on-aspire-app",
                ["client_secret"] = clientSecret,
                ["refresh_token"] = refreshToken
            })
        };
        using var response = await httpClientFactory.CreateClient().SendAsync(request, cancellationToken);
        if (!response.IsSuccessStatusCode)
        {
            throw new ReauthenticationRequiredException("The Keycloak session has expired.");
        }

        var tokens = await response.Content.ReadFromJsonAsync<TokenRefreshResponse>(cancellationToken)
            ?? throw new ReauthenticationRequiredException("Keycloak returned an empty refresh response.");
        if (string.IsNullOrWhiteSpace(tokens.AccessToken))
        {
            throw new ReauthenticationRequiredException("Keycloak did not return a renewed access token.");
        }

        accessToken = tokens.AccessToken;
        if (!string.IsNullOrWhiteSpace(tokens.RefreshToken))
        {
            refreshToken = tokens.RefreshToken;
        }
        accessTokenExpiresAt = DateTimeOffset.UtcNow.AddSeconds(Math.Max(tokens.ExpiresIn, 1));
        return accessToken;
    }

    private sealed class TokenRefreshResponse
    {
        [JsonPropertyName("access_token")]
        public string AccessToken { get; init; } = "";

        [JsonPropertyName("refresh_token")]
        public string? RefreshToken { get; init; }

        [JsonPropertyName("expires_in")]
        public int ExpiresIn { get; init; }
    }
}

public sealed class ReauthenticationRequiredException(string message) : Exception(message);
