module.exports = {
  input: 'build/src/server/index.js',
  output: {
      file: 'build/server.bundle.js',
      format: 'cjs',
      sourcemap: false,
  },
  plugins: [
      require('@rollup/plugin-node-resolve')(),
      require('@rollup/plugin-commonjs')(),
      require('@rollup/plugin-json')(),
  ],
};