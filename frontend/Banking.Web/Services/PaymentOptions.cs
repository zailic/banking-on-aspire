namespace Banking.Web.Services;

public sealed record PaymentAccountOption(string Name, string Label, string CurrencyCode);

public sealed record PaymentBeneficiaryOption(string Name, string Label);
