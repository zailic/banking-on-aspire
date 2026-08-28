using System.Collections.Concurrent;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;

namespace Banking.Web.Auth;

internal sealed class MemoryTicketStore : ITicketStore
{
    private readonly ConcurrentDictionary<string, AuthenticationTicket> tickets = new();

    public Task<string> StoreAsync(AuthenticationTicket ticket)
    {
        RemoveExpiredTickets();
        var key = Guid.NewGuid().ToString("N");
        tickets[key] = ticket;
        return Task.FromResult(key);
    }

    public Task RenewAsync(string key, AuthenticationTicket ticket)
    {
        tickets[key] = ticket;
        return Task.CompletedTask;
    }

    public Task<AuthenticationTicket?> RetrieveAsync(string key)
    {
        if (!tickets.TryGetValue(key, out var ticket))
        {
            return Task.FromResult<AuthenticationTicket?>(null);
        }

        if (ticket.Properties.ExpiresUtc is { } expiresAt && expiresAt <= DateTimeOffset.UtcNow)
        {
            tickets.TryRemove(key, out _);
            return Task.FromResult<AuthenticationTicket?>(null);
        }

        return Task.FromResult<AuthenticationTicket?>(ticket);
    }

    public Task RemoveAsync(string key)
    {
        tickets.TryRemove(key, out _);
        return Task.CompletedTask;
    }

    private void RemoveExpiredTickets()
    {
        var now = DateTimeOffset.UtcNow;
        foreach (var entry in tickets)
        {
            if (entry.Value.Properties.ExpiresUtc is { } expiresAt && expiresAt <= now)
            {
                tickets.TryRemove(entry.Key, out _);
            }
        }
    }
}
