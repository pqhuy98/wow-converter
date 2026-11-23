# WoW Converter Model Viewer

Open an MDX or MDL file from the Explorer with **Open in WoW Converter Model Viewer**, or run the same command from the Command Palette to pick a file. You can also select the viewer under **Open With…** or **Reopen Editor With…**.

The extension bundles wow-converter's Model Viewer. It reads textures and linked models from the model's directory and its ancestors within the containing workspace or converter export folder. Keep the export directory structure when moving models and textures. The viewer runs offline. Its copy-link button produces a VS Code link that reopens the model on the same machine.

## Build and install

From the repository root, install the web UI dependencies using `bun install --cwd webui`. Then run these commands from `vscode-extension`.

```sh
bun run check
bun run package
code --install-extension wow-converter-model-viewer-0.1.0.vsix
```
