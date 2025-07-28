# Huy's Wow-Converter

Export World of Warcraft creatures with weapons straight to Warcraft III's MDX/MDL models with just a few clicks.

Note: the tool works well for Wrath of the Lich King models and below. Later expansions also work but do expect bugs!

---

## Quick Start (No Install Required)

1. Visit **https://wc.quangdel.com/** – the converter UI runs entirely in your browser, nothing to install.
2. Provide your desired NPC character and optionally attached items by giving Wowhead URLs. See prefilled example URLs.
3. Adjust any options you like (animations, size, optimisations, etc.).
4. Click the button **Export Character**.
5. Wait until the export finishes, a download button will appear to download a **ZIP** containing the model (*MDX*/**MDL*) and all required BLP textures.
6. Import the files into your map with World Editor, or open it with WC3 modeling tools like Retera Model Studios.

That’s it — enjoy! ✨

---

## Using the Windows Binaries

Prefer working offline and no waiting queue? Follow these simple steps.

### 1. Get the tools

Download the latest release ZIP and extract it. You will see two files:

| File | Purpose |
|------|---------|
| `wow.export.exe` | Talks to your WoW client / data files. This is forked of https://github.com/Kruithne/wow.export with extra enhacements |
| `wow-converter.exe` | Serves the web UI & does the export  |

### 2. Start **wow.export.exe**

This is my fork of Kruithne's https://github.com/Kruithne/wow.export with the required enhanced capabilities.

1. Open `wow.export.exe`.
2. Select your local **Open Local Installation** or **Use Blizzard CDN** and wait for it to load.
3. Open setting page **Manage Settings**:
    - Turn **ON** the RCP server with *Enable Remote Control Protocol* (leave port **17751**)
    - Make sure **all** “Use Absolute … Paths” options are **OFF**.

### 3. Start **wow-converter**

1. Run `wow-converter.exe`.
2. Wait for the message:
  ```
  ✅ Connected to wow.export RCP at 127.0.0.1:17751
  ✅ Retrieved wow.export asset dir: __________
  ✅ Retrieved wow.export CASC info: __________
  Serving UI web interface at http://127.0.0.1:3001/
  Serving UI web interface at http://127.0.0.1:3001/
  ```

### 4. Export your model

1. Open **http://127.0.0.1:3001/** in your browser.
2. Use the app similarly to **https://wc.quangdel.com/**
3. All exported assets will be stored in the `exported-assets` directory inside the folder where you extracted the app.

---

## ⚙️ Building From Source (Optional)
This section is for experienced programmers who want to build the app from source code. Requires **Node ≥18**, **NPM** and **Git**.

Clone this repository and its wow.export submodule:
```
git clone --recursive https://github.com/pqhuy98/wow-converter
cd wow-converter
npm install
npm run build   # outputs the same binaries found in the release ZIP into the `dist` folder
```


---

## Credits

- Built by me - *Warcraft Sandbox* (<https://www.youtube.com/@WarcraftSandbox>).<br>
- wow.export is forked from the amazing work of **Kruithne**: https://github.com/Kruithne/wow.export
- https://github.com/ChiefOfGxBxL/WC3MapTranslator
- https://github.com/4eb0da/war3-model
- https://github.com/flowtsohg/mdx-m3-viewer
