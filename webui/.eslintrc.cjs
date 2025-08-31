module.exports = {
  extends: [
    "../.eslintrc.cjs"
  ],
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    project: './tsconfig.json',
    tsconfigRootDir: __dirname,
  },
  ignorePatterns: ["**/bin/**/*", "**/dist/**/*", "**/maps/**/*", '**/.eslintrc.cjs', "rollup.config.cjs", "build/**", "wow.export/**/*"],
};
