namespace Banking.Web.Auth;

internal static class LocalReturnUrl
{
    public static string Normalize(string? returnUrl)
    {
        if (string.IsNullOrWhiteSpace(returnUrl)
            || !Uri.TryCreate(returnUrl, UriKind.Relative, out _)
            || returnUrl.StartsWith("//", StringComparison.Ordinal))
        {
            return "/";
        }

        return returnUrl.StartsWith('/') ? returnUrl : $"/{returnUrl}";
    }
}
