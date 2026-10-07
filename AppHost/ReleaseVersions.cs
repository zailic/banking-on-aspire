internal sealed record BankingReleaseVersions(
    string Accounts,
    string Transactions,
    string Contacts,
    string Users,
    string BankingWeb)
{
    public static BankingReleaseVersions FromEnvironment()
    {
        var sharedTag = Environment.GetEnvironmentVariable("BANKING_RELEASE_TAG");
        if (string.IsNullOrWhiteSpace(sharedTag))
        {
            sharedTag = $"aspire-deploy-{DateTimeOffset.UtcNow:yyyyMMddHHmmss}";
        }

        return new BankingReleaseVersions(
            ReadTag("BANKING_ACCOUNTS_IMAGE_TAG", sharedTag),
            ReadTag("BANKING_TRANSACTIONS_IMAGE_TAG", sharedTag),
            ReadTag("BANKING_CONTACTS_IMAGE_TAG", sharedTag),
            ReadTag("BANKING_USERS_IMAGE_TAG", sharedTag),
            ReadTag("BANKING_WEB_IMAGE_TAG", sharedTag));
    }

    private static string ReadTag(string variableName, string fallback)
    {
        var value = Environment.GetEnvironmentVariable(variableName);
        return string.IsNullOrWhiteSpace(value) ? fallback : value;
    }
}
