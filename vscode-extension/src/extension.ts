import { randomBytes } from 'node:crypto';
import { basename } from 'node:path';
import * as vscode from 'vscode';
import { createAssetReader } from './asset-files';

const viewType = 'wowConverter.modelViewer';

export function activate(context: vscode.ExtensionContext) {
  const dist = vscode.Uri.joinPath(context.extensionUri, 'dist');

  async function openModel(uri?: vscode.Uri) {
    const model = uri ?? (await vscode.window.showOpenDialog({
      title: 'Open Warcraft III Model',
      filters: { 'Warcraft III models': ['mdx', 'mdl'] },
    }))?.[0];
    if (!model) return;
    if (model.scheme !== 'file' || !/\.md[xl]$/i.test(model.path)) {
      void vscode.window.showErrorMessage('Select a local Warcraft III MDX or MDL model.');
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
        if (uri.scheme !== 'file') throw new Error('This viewer opens local models.');

        const readAsset = createAssetReader(uri.fsPath, vscode.workspace.getWorkspaceFolder(uri)?.uri.fsPath);
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
        webview.html = viewerHtml(webview, dist, basename(uri.fsPath), link.toString());
      },
    }, { supportsMultipleEditorsPerDocument: true, webviewOptions: { retainContextWhenHidden: true } }),
  );
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
