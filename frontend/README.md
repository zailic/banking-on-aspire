# Applications

This directory owns user-facing applications written in .NET.

`Banking.Web` is the first authenticated vertical slice. It is a .NET 10 Blazor
Web App using interactive server rendering and Fluent UI Blazor v5 RC. It acts
as a BFF: the browser authenticates through Keycloak, while access tokens and
the gRPC calls to Users and Contacts remain on the server.

The BFF renews access tokens server-side with the OIDC refresh token before
expiry. An active Blazor circuit can therefore continue calling protected gRPC
services beyond the five-minute access-token lifetime without forcing logout.

The first page resolves the authenticated user's local profile with
`GetOrCreateCurrentUser`, lists their beneficiaries with `ListContacts`, and
renders both results. Users can add an internal or external beneficiary through
the BFF; the server derives the owner and contact identifier rather than trusting
those values from the browser. Permissions remain enforced by the proto-driven
gRPC authorization interceptors, not by the UI.
