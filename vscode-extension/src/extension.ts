import { randomBytes } from 'node:crypto';
import { basename, isAbsolute } from 'node:path';
import * as vscode from 'vscode';
import { createAssetReader } from './asset-files';

const viewType = 'wowConverter.modelViewer';

interface GitExtension {
  getAPI(version: 1): {
    getRepository(uri: vscode.Uri): { rootUri: vscode.Uri } | null
    toGitUri(uri: vscode.Uri, ref: string): vscode.Uri
  }
}

export function activate(context: vscode.ExtensionContext) {
  const dist = vscode.Uri.joinPath(context.extensionUri, 'dist');

  async function openModel(uri?: vscode.Uri) {
    const model = uri ?? (await vscode.window.showOpenDialog({
      title: 'Open Warcraft III Model',
      filters: { 'Warcraft III models': ['mdx', 'mdl'] },
    }))?.[0];
    if (!model) return;
    const source = modelSource(model);
    if (!source || !/\.md[xl]$/i.test(source.file.path)) {
      void vscode.window.showErrorMessage('Select a Warcraft III MDX or MDL file or Git revision.');
      return;
    }
    await vscode.commands.executeCommand('vscode.openWith', model, viewType);
  }

  context.subscriptions.push(
    vscode.commands.registerCommand('wowConverter.openModel', openModel),
    vscode.window.registerUriHandler({
      handleUri(uri) {
        const model = new URLSearchParams(uri.query).get('model');
        if (uri.path === '/open' && model) return openModel(vscode.Uri.parse(model));
      },
    }),
    vscode.window.registerCustomEditorProvider(viewType, {
      openCustomDocument(uri) {
        return { uri, dispose() {} };
      },
      resolveCustomEditor(document, panel) {
        const { uri } = document;
        const source = modelSource(uri);
        if (!source) throw new Error('This viewer opens local models and Git revisions.');

        const { file, readRevision } = source;
        const readAsset = createAssetReader(file.fsPath, vscode.workspace.getWorkspaceFolder(file)?.uri.fsPath, readRevision);
        const link = vscode.Uri.from({
          scheme: vscode.env.uriScheme,
          authority: context.extension.id,
          path: '/open',
          query: new URLSearchParams({ model: uri.toString() }).toString(),
        });

        const { webview } = panel;
        webview.options = { enableScripts: true, localResourceRoots: [dist] };
        let disposed = false;
        const messages = webview.onDidReceiveMessage(async (request: unknown) => {
          if (!request || typeof request !== 'object' || !('id' in request) || typeof request.id !== 'number'
            || !('kind' in request)) return;
          const reply: { id: number; value?: ArrayBuffer; error?: string } = { id: request.id };
          try {
            if (request.kind === 'read' && 'path' in request && typeof request.path === 'string') {
              reply.value = await readAsset(request.path);
            } else if (request.kind === 'copy' && 'text' in request && typeof request.text === 'string') {
              await vscode.env.clipboard.writeText(request.text);
            } else {
              return;
            }
          } catch (err) {
            reply.error = err instanceof Error ? err.message : 'Viewer request failed.';
          }
          if (!disposed) await webview.postMessage(reply);
        });
        panel.onDidDispose(() => {
          disposed = true;
          messages.dispose();
        });
        webview.html = viewerHtml(webview, dist, basename(file.fsPath), link.toString());
      },
    }, { supportsMultipleEditorsPerDocument: true, webviewOptions: { retainContextWhenHidden: true } }),
  );
}

function modelSource(uri: vscode.Uri): { file: vscode.Uri; readRevision?: (file: string) => Promise<Uint8Array | undefined> } | undefined {
  if (uri.scheme === 'file') return { file: uri };
  if (uri.scheme !== 'git') return undefined;

  // VS Code's Git provider stores the underlying path and the revision in the URI query.
  let query: unknown;
  try {
    query = JSON.parse(uri.query);
  } catch {
    return undefined;
  }
  if (!query || typeof query !== 'object'
    || !('path' in query) || typeof query.path !== 'string' || !isAbsolute(query.path)
    || !('ref' in query) || typeof query.ref !== 'string') return undefined;

  const file = vscode.Uri.file(query.path);
  const { ref } = query;
  return {
    file,
    async readRevision(path) {
      let resource = uri;
      if (path !== file.fsPath) {
        const git = vscode.extensions.getExtension<GitExtension>('vscode.git')?.exports?.getAPI(1);
        const modelRepository = git?.getRepository(file);
        const asset = vscode.Uri.file(path);
        const assetRepository = git?.getRepository(asset);
        if (!git || !modelRepository || assetRepository?.rootUri.toString() !== modelRepository.rootUri.toString()) return undefined;
        resource = git.toGitUri(asset, ref);
      }
      try {
        return await vscode.workspace.fs.readFile(resource);
      } catch (error) {
        if (error instanceof vscode.FileSystemError && error.code === 'FileNotFound') return undefined;
        throw error;
      }
    },
  };
}

function viewerHtml(webview: vscode.Webview, dist: vscode.Uri, model: string, link: string) {
  const nonce = randomBytes(18).toString('base64');
  const uri = (name: string) => webview.asWebviewUri(vscode.Uri.joinPath(dist, name));
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'nonce-${nonce}'; style-src ${webview.cspSource} 'unsafe-inline'; img-src blob: data:;">
  <title>WoW Converter Model Viewer</title>
  <link rel="stylesheet" href="${uri('viewer.css')}">
</head>
<body>
  <div id="root" data-model="${encodeURIComponent(model)}" data-link="${encodeURIComponent(link)}"></div>
  <script nonce="${nonce}" src="${uri('viewer.js')}"></script>
</body>
</html>`;
}
