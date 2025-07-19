# WoW Converter

A tool for converting World of Warcraft models and characters.

## Build Options

### Development
```bash
npm run dev
```

### Build Executable

There are several build options depending on your needs:

#### 1. Standard Build (requires resources)
```bash
npm run build:exe
```
Creates `wow-converter.exe` that requires the `bin` and `dist/webui` folders to be present.

#### 2. Complete Distribution Package
```bash
npm run build:dist
```
Creates a complete distribution package in `dist-package/` that includes:
- `wow-converter-complete.exe` - The executable
- `bin/` - Native bindings and resources
- `webui/` - Web interface files

This package can be copied to any location and will work standalone.

#### 3. Standalone Executable (experimental)
```bash
npm run build:exe:standalone
```
Creates `wow-converter-standalone.exe` that attempts to bundle everything into a single file.

## Usage

### Running the Server
1. Build the complete distribution: `npm run build:dist`
2. Copy the `dist-package` folder to your desired location
3. Run `wow-converter-complete.exe`
4. Open http://localhost:3001 in your browser

### API Endpoints
- `POST /export/character` - Export a character model
- `GET /` - Web interface

## Troubleshooting

If you get module resolution errors when running the executable:
1. Make sure you're using the complete distribution package (`npm run build:dist`)
2. Ensure all files in the `dist-package` folder are present
3. The executable must be run from the same directory as the `bin` and `webui` folders 