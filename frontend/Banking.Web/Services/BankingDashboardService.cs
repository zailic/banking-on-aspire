using Banking.Contracts.Accounts.V1;
using Banking.Contracts.Contacts.V1;
using Banking.Contracts.Transactions.V1;
using Banking.Contracts.Users.V1;
using Banking.Web.Auth;
using Grpc.Core;
using Grpc.Net.Client;
using System.ComponentModel.DataAnnotations;
using System.Globalization;
using System.Text;

namespace Banking.Web.Services;

public sealed class BankingDashboardService(IConfiguration configuration, AccessTokenProvider accessTokenProvider)
{
    public async Task<BankingDashboard> LoadAsync(CancellationToken cancellationToken = default)
    {
        using var usersChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:UsersUrl"));
        var users = new UsersService.UsersServiceClient(usersChannel);
        var user = await ExecuteAuthenticatedAsync(
            headers => users.GetOrCreateCurrentUserAsync(
                new GetOrCreateCurrentUserRequest(), headers, cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);

        using var accountsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:AccountsUrl"));
        var accounts = new AccountsService.AccountsServiceClient(accountsChannel);
        var accountsResponse = await ExecuteAuthenticatedAsync(
            headers => accounts.ListAccountsAsync(
                new ListAccountsRequest { Parent = user.Name, PageSize = 50 }, headers,
                cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);

        using var contactsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:ContactsUrl"));
        var contacts = new ContactsService.ContactsServiceClient(contactsChannel);
        var contactsResponse = await ExecuteAuthenticatedAsync(
            headers => contacts.ListContactsAsync(
                new ListContactsRequest { Parent = user.Name, PageSize = 50 }, headers,
                cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);

        using var transactionsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:TransactionsUrl"));
        var transactions = new TransactionsService.TransactionsServiceClient(transactionsChannel);
        var transactionsResponse = await ExecuteAuthenticatedAsync(
            headers => transactions.ListTransactionsAsync(
                new ListTransactionsRequest { Parent = user.Name, PageSize = 10 }, headers,
                cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);

        return new BankingDashboard(
            user,
            [.. accountsResponse.Accounts],
            [.. contactsResponse.Contacts],
            [.. transactionsResponse.Transactions]);
    }

    public async Task<Contact> CreateContactAsync(
        CreateContactCommand command,
        CancellationToken cancellationToken = default)
    {
        Validator.ValidateObject(command, new ValidationContext(command), validateAllProperties: true);
        using var usersChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:UsersUrl"));
        var users = new UsersService.UsersServiceClient(usersChannel);
        var user = await ExecuteAuthenticatedAsync(
            headers => users.GetOrCreateCurrentUserAsync(
                new GetOrCreateCurrentUserRequest(), headers, cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);

        var contact = new Contact { DisplayName = command.DisplayName.Trim() };
        if (command.DestinationType == ContactDestinationType.Internal)
        {
            contact.InternalAccount = command.InternalAccount.Trim();
        }
        else
        {
            contact.ExternalAccount = new ExternalBankAccount
            {
                RoutingNumber = command.RoutingNumber.Trim(),
                AccountNumber = command.AccountNumber.Trim()
            };
        }

        using var contactsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:ContactsUrl"));
        var contacts = new ContactsService.ContactsServiceClient(contactsChannel);
        return await ExecuteAuthenticatedAsync(
            headers => contacts.CreateContactAsync(
                new CreateContactRequest
                {
                    Parent = user.Name,
                    ContactId = CreateContactId(command.DisplayName),
                    Contact = contact
                },
                headers,
                cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);
    }

    public async Task<Account> CreateAccountAsync(
        CreateAccountCommand command,
        CancellationToken cancellationToken = default)
    {
        Validator.ValidateObject(command, new ValidationContext(command), validateAllProperties: true);
        using var accountsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:AccountsUrl"));
        var accounts = new AccountsService.AccountsServiceClient(accountsChannel);
        var request = new CreateAccountRequest
        {
            Parent = command.Owner.Trim(),
            Account = new Account
            {
                DisplayName = command.DisplayName.Trim(),
                Type = command.Type,
                CurrencyCode = command.CurrencyCode.Trim().ToUpperInvariant()
            }
        };
        if (!string.IsNullOrWhiteSpace(command.AccountId))
        {
            request.AccountId = command.AccountId.Trim();
        }

        return await ExecuteAuthenticatedAsync(
            headers => accounts.CreateAccountAsync(
                request, headers, cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);
    }

    public async Task<Payment> SendPaymentAsync(
        SendPaymentCommand command,
        CancellationToken cancellationToken = default)
    {
        Validator.ValidateObject(command, new ValidationContext(command), validateAllProperties: true);
        var amount = decimal.Round(command.Amount, 2, MidpointRounding.AwayFromZero);
        var units = decimal.ToInt64(decimal.Truncate(amount));
        var nanos = decimal.ToInt32((amount - units) * 1_000_000_000m);
        using var accountsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:AccountsUrl"));
        var accounts = new AccountsService.AccountsServiceClient(accountsChannel);
        return await ExecuteAuthenticatedAsync(
            headers => accounts.SendPaymentAsync(
                new SendPaymentRequest
                {
                    Parent = command.SourceAccount.Trim(),
                    Beneficiary = command.Beneficiary.Trim(),
                    Amount = new Google.Type.Money
                    {
                        CurrencyCode = command.CurrencyCode.Trim().ToUpperInvariant(),
                        Units = units,
                        Nanos = nanos
                    },
                    Reference = command.Reference.Trim(),
                    RequestId = string.IsNullOrWhiteSpace(command.RequestId)
                        ? Guid.NewGuid().ToString("N")
                        : command.RequestId.Trim()
                },
                headers,
                cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);
    }

    public async Task<Deposit> DepositFundsAsync(
        DepositFundsCommand command,
        CancellationToken cancellationToken = default)
    {
        Validator.ValidateObject(command, new ValidationContext(command), validateAllProperties: true);
        var amount = decimal.Round(command.Amount, 2, MidpointRounding.AwayFromZero);
        var units = decimal.ToInt64(decimal.Truncate(amount));
        var nanos = decimal.ToInt32((amount - units) * 1_000_000_000m);
        using var accountsChannel = GrpcChannel.ForAddress(GetGrpcAddress("Backend:AccountsUrl"));
        var accounts = new AccountsService.AccountsServiceClient(accountsChannel);
        return await ExecuteAuthenticatedAsync(
            headers => accounts.DepositFundsAsync(
                new DepositFundsRequest
                {
                    Parent = command.Account.Trim(),
                    Amount = new Google.Type.Money
                    {
                        CurrencyCode = command.CurrencyCode.Trim().ToUpperInvariant(),
                        Units = units,
                        Nanos = nanos
                    },
                    Reference = command.Reference.Trim(),
                    RequestId = string.IsNullOrWhiteSpace(command.RequestId)
                        ? Guid.NewGuid().ToString("N")
                        : command.RequestId.Trim()
                },
                headers,
                cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);
    }

    private async Task<T> ExecuteAuthenticatedAsync<T>(
        Func<Metadata, Task<T>> operation,
        CancellationToken cancellationToken)
    {
        var token = await accessTokenProvider.GetAccessTokenAsync(cancellationToken: cancellationToken);
        try
        {
            return await operation(CreateAuthorizationHeaders(token));
        }
        catch (RpcException exception) when (exception.StatusCode == StatusCode.Unauthenticated)
        {
            token = await accessTokenProvider.GetAccessTokenAsync(
                forceRefresh: true,
                cancellationToken: cancellationToken);
            return await operation(CreateAuthorizationHeaders(token));
        }
    }

    private static Metadata CreateAuthorizationHeaders(string accessToken) =>
        new() { { "Authorization", $"Bearer {accessToken}" } };

    private static string CreateContactId(string displayName)
    {
        var normalized = displayName.Trim().ToLowerInvariant().Normalize(NormalizationForm.FormD);
        var slug = new StringBuilder();
        var pendingHyphen = false;
        foreach (var character in normalized)
        {
            if (CharUnicodeInfo.GetUnicodeCategory(character) == UnicodeCategory.NonSpacingMark)
            {
                continue;
            }
            if (character is >= 'a' and <= 'z' or >= '0' and <= '9')
            {
                if (pendingHyphen && slug.Length > 0) slug.Append('-');
                slug.Append(character);
                pendingHyphen = false;
            }
            else
            {
                pendingHyphen = true;
            }
        }

        var prefix = slug.Length == 0 || !char.IsLetter(slug[0]) ? "contact" : slug.ToString();
        if (prefix.Length > 48) prefix = prefix[..48].TrimEnd('-');
        return $"{prefix}-{Guid.NewGuid():N}"[..Math.Min(prefix.Length + 9, 63)];
    }

    private Uri GetGrpcAddress(string key)
    {
        var value = configuration[key]
            ?? throw new InvalidOperationException($"{key} is required.");
        if (value.StartsWith("grpc://", StringComparison.OrdinalIgnoreCase))
        {
            value = $"http://{value[7..]}";
        }
        return new Uri(value);
    }
}

public sealed record BankingDashboard(
    User User,
    IReadOnlyList<Account> Accounts,
    IReadOnlyList<Contact> Contacts,
    IReadOnlyList<Transaction> Transactions);

public enum ContactDestinationType
{
    Internal,
    External
}

public sealed class CreateContactCommand : IValidatableObject
{
    [Required, StringLength(120)]
    public string DisplayName { get; set; } = "";

    public ContactDestinationType DestinationType { get; set; }

    public string InternalAccount { get; set; } = "";

    public string RoutingNumber { get; set; } = "";

    public string AccountNumber { get; set; } = "";

    public IEnumerable<ValidationResult> Validate(ValidationContext validationContext)
    {
        if (DestinationType == ContactDestinationType.Internal)
        {
            if (string.IsNullOrWhiteSpace(InternalAccount)
                || !InternalAccount.Trim().StartsWith("accounts/", StringComparison.Ordinal))
            {
                yield return new ValidationResult(
                    "Use an account resource name such as accounts/checking-01.",
                    [nameof(InternalAccount)]);
            }
            yield break;
        }

        if (string.IsNullOrWhiteSpace(RoutingNumber))
        {
            yield return new ValidationResult("Routing number is required.", [nameof(RoutingNumber)]);
        }
        if (string.IsNullOrWhiteSpace(AccountNumber))
        {
            yield return new ValidationResult("Account number is required.", [nameof(AccountNumber)]);
        }
    }
}

public sealed class CreateAccountCommand
{
    [Required, RegularExpression(@"^users/[^/]+$", ErrorMessage = "Use a user resource such as users/alice.")]
    public string Owner { get; set; } = "";

    [Required, StringLength(120)]
    public string DisplayName { get; set; } = "";

    [EnumDataType(typeof(AccountType))]
    public AccountType Type { get; set; } = AccountType.Checking;

    [Required, RegularExpression(@"^[A-Za-z]{3}$", ErrorMessage = "Use a three-letter currency code such as RON or EUR.")]
    public string CurrencyCode { get; set; } = "RON";

    [RegularExpression(@"^$|^[a-z][a-z0-9-]{0,62}$", ErrorMessage = "Use lowercase letters, digits and hyphens; start with a letter.")]
    public string AccountId { get; set; } = "";
}

public sealed class SendPaymentCommand
{
    [Required, RegularExpression(@"^accounts/[^/]+$", ErrorMessage = "Select a source account.")]
    public string SourceAccount { get; set; } = "";

    [Required, RegularExpression(@"^users/[^/]+/contacts/[^/]+$", ErrorMessage = "Select a beneficiary.")]
    public string Beneficiary { get; set; } = "";

    [Range(typeof(decimal), "0.01", "999999999999999999", ErrorMessage = "Enter an amount greater than zero.")]
    public decimal Amount { get; set; }

    [Required, RegularExpression(@"^[A-Za-z]{3}$")]
    public string CurrencyCode { get; set; } = "RON";

    [StringLength(140)]
    public string Reference { get; set; } = "";

    public string RequestId { get; set; } = "";
}

public sealed class DepositFundsCommand
{
    [Required, RegularExpression(@"^accounts/[^/]+$", ErrorMessage = "Select an account.")]
    public string Account { get; set; } = "";

    [Range(typeof(decimal), "0.01", "999999999999999999", ErrorMessage = "Enter an amount greater than zero.")]
    public decimal Amount { get; set; }

    [Required, RegularExpression(@"^[A-Za-z]{3}$")]
    public string CurrencyCode { get; set; } = "RON";

    [StringLength(140)]
    public string Reference { get; set; } = "Demo cash-in";

    public string RequestId { get; set; } = "";
}
