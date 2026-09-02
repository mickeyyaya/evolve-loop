// ESLint flat config for the repo's browser-side assets (today: the single
// embedded dashboard script, go/internal/dashboard/static/app.js). The commit
// gate's Node lane (internal/commitgate/lanes.go laneNode) runs eslint over
// changed .js files; ESLint ≥ 9 requires this file to exist. Core rules only —
// no plugins, no package.json — so `npx eslint` works on a clean machine.
export default [
  {
    files: ["**/*.js"],
    ignores: ["**/node_modules/**", "landing/dist/**"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "script",
      globals: {
        document: "readonly", window: "readonly", location: "readonly",
        fetch: "readonly", EventSource: "readonly", setInterval: "readonly",
        alert: "readonly", console: "readonly", Date: "readonly",
      },
    },
    rules: {
      "no-undef": "error",
      "no-unused-vars": ["error", { args: "none" }],
      "no-implied-eval": "error",
      "no-eval": "error",
      "no-new-func": "error",
      "eqeqeq": ["error", "smart"],
      "no-var": "error",
      "prefer-const": "error",
    },
  },
];
