(function () {
  "use strict";

  var root = document.documentElement;
  var english = root.lang.toLowerCase().indexOf("en") === 0;
  var storedTheme = localStorage.getItem("mw-identity-theme");
  if (storedTheme === "light" || storedTheme === "dark") {
    root.dataset.theme = storedTheme;
  }

  function isDark() {
    return (
      root.dataset.theme === "dark" ||
      (!root.dataset.theme && matchMedia("(prefers-color-scheme: dark)").matches)
    );
  }

  function syncThemeControls() {
    var dark = isDark();
    document.querySelectorAll("[data-theme-toggle]").forEach(function (button) {
      button.setAttribute("aria-pressed", String(dark));
      button.setAttribute(
        "aria-label",
        dark
          ? english
            ? "Use light theme"
            : "Usar tema claro"
          : english
            ? "Use dark theme"
            : "Usar tema escuro",
      );
      var use = button.querySelector("use");
      if (use) use.setAttribute("href", "/static/icons.svg#" + (dark ? "sun" : "moon"));
    });
    var themeColor = document.querySelector('meta[name="theme-color"]');
    if (themeColor) themeColor.content = dark ? "#111214" : "#f7f7f5";
  }

  document.querySelectorAll("[data-theme-toggle]").forEach(function (button) {
    button.addEventListener("click", function () {
      root.dataset.theme = isDark() ? "light" : "dark";
      localStorage.setItem("mw-identity-theme", root.dataset.theme);
      syncThemeControls();
    });
  });

  var languageLink = document.querySelector("[data-language-link]");
  if (languageLink && location.search) languageLink.href += location.search;

  syncThemeControls();
})();
