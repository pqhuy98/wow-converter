# Shot export

Screenshot an exported model from the running wow-converter.

1. Run `bun scripts/shot-export.ts <asset-path>` in the wow-converter repo. The server is http://127.0.0.1:3001. Pass `--seq` when a sequence other than Stand is needed, and `--view front` to take one side instead of all six.
2. Read the PNG paths the script prints.
