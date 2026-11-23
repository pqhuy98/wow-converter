import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';

const extension = resolve(import.meta.dir, '..');
const webui = resolve(extension, '../webui');
const dist = join(extension, 'dist');
const requireWebui = createRequire(join(webui, 'package.json'));
const packageDirs = new Set<string>();

for (const options of [
  {
    entrypoints: [join(extension, 'src/extension.ts')],
    target: 'node',
    format: 'cjs',
    naming: 'extension.cjs',
    external: ['vscode'],
  },
  {
    entrypoints: [join(extension, 'src/webview.tsx')],
    target: 'browser',
    format: 'iife',
    naming: 'viewer.js',
    minify: true,
    define: { 'process.env.NODE_ENV': '"production"' },
    plugins: [{
      name: 'wow-converter-webui',
      setup(build) {
        build.onResolve({ filter: /^@wow-converter\/model-viewer$/ }, () => ({
          path: join(webui, 'components/common/model-viewer/model-viewer.tsx'),
        }));
        build.onResolve({ filter: /^(@\/|@api\/)/ }, ({ path }) => ({
          path: Bun.resolveSync(join(webui, path.replace(/^@\//, '').replace(/^@api\//, 'lib/api/')), webui),
        }));
        build.onResolve({ filter: /^react(?:-dom)?(?:\/.*)?$/ }, ({ path }) => ({
          path: requireWebui.resolve(path),
        }));
      },
    }],
  },
] satisfies Bun.BuildConfig[]) {
  const result = await Bun.build({ ...options, outdir: dist, metafile: true });
  if (!result.success) throw new AggregateError(result.logs, 'Viewer build failed');
  for (const input of Object.keys(result.metafile?.inputs ?? {})) {
    const dependency = input.replaceAll('\\', '/').match(/^(.*\/node_modules\/(?:@[^/]+\/)?[^/]+)\//);
    if (dependency) packageDirs.add(resolve(dependency[1]));
  }
}

const postcss = requireWebui('postcss');
const tailwind = requireWebui('tailwindcss');
const loadConfig = requireWebui('tailwindcss/loadConfig');
const config = loadConfig(join(webui, 'tailwind.config.ts'));
config.content = [
  join(webui, 'components/common/model-viewer/*.tsx'),
  join(webui, 'components/common/tooltip-help.tsx'),
  join(webui, 'components/ui/{button,tooltip}.tsx'),
  join(webui, 'app/viewer/viewer-content.tsx'),
  join(extension, 'src/webview.tsx'),
].map((path) => path.replaceAll('\\', '/'));

const globals = join(webui, 'app/globals.css');
const css = (await Bun.file(globals).text()).replace(/^@import.*wow-expansion-icons\.css.*$/m, '');
const { css: viewerCss } = await postcss([tailwind(config)]).process(css, { from: globals });
await Bun.write(join(dist, 'viewer.css'), `${viewerCss}\nhtml, body { padding: 0; margin: 0; overflow: hidden; }\n`);

const licenses = [];
const licenseFiles = new Bun.Glob('{LICENSE,LICENSE.*,LICENCE,LICENCE.*,COPYING,COPYING.*}');
for (const dir of [...packageDirs].sort()) {
  const { name, version } = await Bun.file(join(dir, 'package.json')).json();
  for await (const file of licenseFiles.scan({ cwd: dir })) {
    licenses.push(`${name} ${version}\n\n${await Bun.file(join(dir, file)).text()}`);
  }
}
await Bun.write(join(dist, 'NOTICES.txt'), licenses.join('\n\n'));
