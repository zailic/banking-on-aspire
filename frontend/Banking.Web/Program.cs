using Banking.Web.Auth;
using Banking.Web.Components;
using Banking.Web.Services;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.Authentication.OpenIdConnect;
using Microsoft.FluentUI.AspNetCore.Components;
using System.Security.Claims;

var builder = WebApplication.CreateBuilder(args);
var authenticationTickets = new MemoryTicketStore();

builder.AddServiceDefaults();
builder.Services.AddRazorComponents().AddInteractiveServerComponents();
builder.Services.AddFluentUIComponents();
builder.Services.AddCascadingAuthenticationState();
builder.Services.AddHttpContextAccessor();
builder.Services.AddHttpClient();
builder.Services.AddScoped<AccessTokenProvider>();
builder.Services.AddScoped<BankingDashboardService>();
builder.Services.AddSingleton(authenticationTickets);

builder.Services
    .AddAuthentication(options =>
    {
        options.DefaultScheme = CookieAuthenticationDefaults.AuthenticationScheme;
        options.DefaultChallengeScheme = OpenIdConnectDefaults.AuthenticationScheme;
    })
    .AddCookie(options =>
    {
        options.Cookie.Name = "banking-web";
        options.SessionStore = authenticationTickets;
    })
    .AddOpenIdConnect(options =>
    {
        var keycloakBaseUrl = builder.Configuration["Authentication:KeycloakBaseUrl"]
            ?? throw new InvalidOperationException("Authentication:KeycloakBaseUrl is required.");
        var keycloakIssuerBaseUrl = builder.Configuration["Authentication:KeycloakIssuerBaseUrl"]
            ?? keycloakBaseUrl;
        options.Authority = $"{keycloakIssuerBaseUrl.TrimEnd('/')}/realms/banking-on-aspire";
        options.MetadataAddress =
            $"{keycloakBaseUrl.TrimEnd('/')}/realms/banking-on-aspire/.well-known/openid-configuration";
        options.RequireHttpsMetadata = Uri.TryCreate(keycloakBaseUrl, UriKind.Absolute, out var keycloakUri) &&
                                       keycloakUri.Scheme == Uri.UriSchemeHttps;
        options.ClientId = "banking-on-aspire-app";
        options.ClientSecret = builder.Configuration["Authentication:ClientSecret"]
            ?? throw new InvalidOperationException("Authentication:ClientSecret is required.");
        options.ResponseType = "code";
        options.SaveTokens = true;
        options.GetClaimsFromUserInfoEndpoint = true;
        options.MapInboundClaims = false;
        options.TokenValidationParameters.NameClaimType = "preferred_username";
        options.TokenValidationParameters.RoleClaimType = ClaimTypes.Role;
        options.Scope.Add("profile");
        options.Scope.Add("email");
        options.Events.OnTicketReceived = context =>
        {
            var identity = context.Principal?.Identity as ClaimsIdentity;
            var accessToken = context.Properties?.GetTokenValue("access_token");
            if (identity is not null && !string.IsNullOrWhiteSpace(accessToken))
            {
                KeycloakClientRoles.AddTo(identity, accessToken, "banking-on-aspire-app");
            }
            return Task.CompletedTask;
        };
    });
builder.Services.AddAuthorization(options =>
{
    options.AddPolicy("AccountsCreate", policy => policy.RequireRole("accounts.create"));
    options.AddPolicy("AccountsDeposit", policy => policy.RequireRole("accounts.deposit"));
    options.AddPolicy("PaymentsSend", policy => policy.RequireRole("payments.send"));
});

var app = builder.Build();

if (!app.Environment.IsDevelopment())
{
    app.UseExceptionHandler("/Error", createScopeForErrors: true);
    app.UseHsts();
}
app.UseStatusCodePagesWithReExecute("/not-found", createScopeForStatusCodePages: true);
app.UseHttpsRedirection();
app.UseAuthentication();
app.UseAuthorization();
app.UseAntiforgery();

app.MapGet("/authentication/login", (HttpContext context, string? returnUrl) =>
{
    // Cookies are scoped by host, not port. Remove the previous large/chunked
    // cookie before redirecting from Banking Web to Keycloak on localhost.
    foreach (var cookieName in context.Request.Cookies.Keys
                 .Where(name => name.StartsWith("banking-web", StringComparison.Ordinal)))
    {
        context.Response.Cookies.Delete(cookieName);
    }

    return Results.Challenge(
        new AuthenticationProperties { RedirectUri = LocalReturnUrl.Normalize(returnUrl) },
        [OpenIdConnectDefaults.AuthenticationScheme]);
}).AllowAnonymous();

app.MapPost("/authentication/logout", () => Results.SignOut(
    new AuthenticationProperties { RedirectUri = "/" },
    [CookieAuthenticationDefaults.AuthenticationScheme, OpenIdConnectDefaults.AuthenticationScheme]));

app.MapStaticAssets();
app.MapRazorComponents<App>().AddInteractiveServerRenderMode();
app.MapDefaultEndpoints();

app.Run();
