module.exports = {
  input: 'build/src/server/index.js',
  output: {
    file: 'build/server.bundle.js',
    format: 'cjs',
    sourcemap: false,
  },
  external: [
    'sharp',
  ],
  plugins: [
    require('@rollup/plugin-node-resolve')({ preferBuiltins: true }),
    require('@rollup/plugin-commonjs')({}),
    require('@rollup/plugin-json')(),
  ],
};