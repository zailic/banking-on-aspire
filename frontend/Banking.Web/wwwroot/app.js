window.BankingWeb = {
    getTheme() {
        return localStorage.getItem("banking-web-theme") ?? "system";
    },

    setTheme(mode) {
        const themeApi = globalThis.Blazor?.FluentUI?.Blazor?.Utilities?.Theme
            ?? globalThis.Blazor?.theme;

        if (!themeApi?.setThemeMode) {
            console.warn("Fluent UI theme API is not available.");
            return;
        }

        themeApi.setThemeMode(mode);
        localStorage.setItem("banking-web-theme", mode);
    },

    navigate(url) {
        window.location.assign(url);
    },

    submitForm(id) {
        document.getElementById(id)?.requestSubmit();
    }
};
