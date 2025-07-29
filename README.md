# Huy's Wow-Converter

Export models of World of Warcraft NPCs straight to Warcraft III's MDX/MDL models with just a few clicks.

Demo video: https://youtu.be/FLgBKT7c2TI?si=wdPLJ6pVaEVFE5iS

HiveWorkshop thread: https://www.hiveworkshop.com/threads/wow-converter-export-wow-npcs-to-warcraft-3.363033/

![screenshot 1](https://github.com/pqhuy98/wow-converter/blob/main/docs/elwin-forest-1.jpg?raw=true)

![screenshot 2](https://github.com/pqhuy98/wow-converter/blob/main/docs/icecrown-1.jpg?raw=true)

Note: the tool works well for Wrath of the Lich King models and below. Later expansions will work but do expect some bugs!

Give this repo a ⭐ if you find it useful or interesting!

---

## Quick Start (No Install Required)

1. Visit **https://wc.quangdel.com/** – the converter UI runs entirely in your browser, nothing to install.
2. Provide your desired NPC character and optionally attached items by giving Wowhead URLs. See prefilled example URLs.
3. Adjust any options you like (animations, size, optimisations, etc.).
4. Click the button **Export Character**.
5. Wait until the export finishes, a download button will appear to download a **ZIP** containing the model (MDX/MDL) and all required BLP textures.
6. Import the files into your map with World Editor, or open it with WC3 modeling tools like Retera Model Studios.

That’s it - enjoy! ✨

---

## Using the Windows Binaries

Prefer working offline, fast, without waiting queue, and no constant ZIP download and extraction? Follow these simple steps to run the app locally.

### 1. Get the tools

Download the latest release ZIP here: https://github.com/pqhuy98/wow-converter/releases and extract it. You will see the two main files among many other files:

| File | Purpose |
|------|---------|
| `wow.export.exe` | Talks to your WoW client / data files. This is forked of https://github.com/Kruithne/wow.export with extra enhacements |
| `wow-converter.exe` | Serves the web UI & does the export  |

### 2. Start **wow.export**

This is my fork of Kruithne's https://github.com/Kruithne/wow.export with the required enhanced capabilities. You will need to keep it open and turn on its RCP server functionality.

1. Open `wow.export.exe`.
2. Select **Open Local Installation** and your local WoW installation, or **Use Blizzard CDN** if you don't have one. Wait for it to load.
3. Open setting page **Manage Settings**:
    - Turn **ON** the RCP server with `Enable Remote Control Protocol` (leave default port **17751**)
    - Make sure **all** `Use Absolute … Paths` options are **OFF**.

![wow.export.exe](https://github.com/pqhuy98/wow-converter/blob/main/docs/wow.export-1.jpg?raw=true)


### 3. Start **wow-converter**

1. Run `wow-converter.exe`. A command line window will open displaying all app messages.
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
3. All exported assets will be stored in the `exported-assets` directory inside the folder where you extracted the app. This is better than hosted version because you won't need to download or extract any ZIP file. 

---

## Building From Source (Optional)
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
- Exported assets are from World of Warcraft, Blizzard Entertainment.
- https://github.com/ChiefOfGxBxL/WC3MapTranslator
- https://github.com/4eb0da/war3-model
- https://github.com/flowtsohg/mdx-m3-viewer
