
const glob = require('glob');
const path = require('path');

module.exports = {
  input: {
    'server.bundle': 'src/server/index.ts',
      ...getWorkerFiles(),
  },
  output: {
    dir: 'build',
    format: 'cjs',
    sourcemap: true,
    entryFileNames: '[name].cjs',
    chunkFileNames: 'chunks/[name]-[hash].cjs',
  },
  external: [
    'sharp',
  ],
  plugins: [
    require('@rollup/plugin-node-resolve')({ preferBuiltins: true }),
    require('@rollup/plugin-commonjs')({}),
    require('@rollup/plugin-json')(),
    require('@rollup/plugin-typescript')()
  ],
};

// Auto-discover all *.worker.ts files
function getWorkerFiles() {
  const workerFiles = glob.sync('src/**/*.worker.ts');
  const basenames = new Map();
  
  for (const file of workerFiles) {
    const basename = path.basename(file).replace(/.(ts|js|cjs)$/, '');
    if (basenames.has(basename)) {
      throw new Error(`Duplicate worker basename "${basename}": ${basenames.get(basename)} and ${file}`);
    }
    basenames.set(basename, file);
  }

  const result = Object.fromEntries(basenames.entries());
  console.log('workerFiles:', result);

  return result;
}