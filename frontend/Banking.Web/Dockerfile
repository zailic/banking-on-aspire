ARG DOTNET_SDK_VERSION=10.0.401
ARG DOTNET_RUNTIME_VERSION=10.0.12
ARG VERSION=0.0.0-dev
ARG VCS_REF=unknown

FROM mcr.microsoft.com/dotnet/sdk:${DOTNET_SDK_VERSION} AS build

WORKDIR /src

COPY platform/dotnet/Banking.Contracts/Banking.Contracts.csproj platform/dotnet/Banking.Contracts/
COPY platform/dotnet/Banking.ServiceDefaults/Banking.ServiceDefaults.csproj platform/dotnet/Banking.ServiceDefaults/
COPY frontend/Banking.Web/Banking.Web.csproj frontend/Banking.Web/
RUN dotnet restore frontend/Banking.Web/Banking.Web.csproj

COPY platform/gen/csharp ./platform/gen/csharp
COPY platform/dotnet ./platform/dotnet
COPY frontend/Banking.Web ./frontend/Banking.Web
RUN dotnet publish frontend/Banking.Web/Banking.Web.csproj \
    --configuration Release \
    --output /app/publish \
    /p:UseAppHost=false

# Blazor's runtime files are supplied by Microsoft.AspNetCore.App.Internal.Assets.
# Fail the image build instead of publishing an application with a broken UI.
RUN test -f /app/publish/wwwroot/_framework/blazor.web.js

FROM mcr.microsoft.com/dotnet/aspnet:${DOTNET_RUNTIME_VERSION}

ARG VERSION=0.0.0-dev
ARG VCS_REF=unknown

LABEL org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$VCS_REF

WORKDIR /app
COPY --from=build /app/publish .

USER $APP_UID
EXPOSE 8080
ENTRYPOINT ["dotnet", "Banking.Web.dll"]
