import { chromium } from 'playwright';

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:3001/';
const NPC_URL = 'https://www.wowhead.com/wotlk/npc=36855/lady-deathwhisper';
const ITEM_URL = 'https://www.wowhead.com/item=120978/ashbringer';
const SCREENSHOT_PATH = process.env.SCREENSHOT_PATH || '/workspace/docs/lady-deathwhisper.png';

async function waitForServer(url, timeoutMs = 120_000) {
	const start = Date.now();
	while (Date.now() - start < timeoutMs) {
		try {
			const res = await fetch(url, { method: 'GET' });
			if (res.ok) return true;
		} catch (_) {
			// ignore
		}
		await new Promise((r) => setTimeout(r, 1000));
	}
	throw new Error(`Server not ready at ${url} after ${timeoutMs}ms`);
}

async function run() {
	console.log('Waiting for app to be ready at', APP_URL);
	await waitForServer(APP_URL);

	const browser = await chromium.launch({ headless: true });
	const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });
	const page = await context.newPage();
	await page.goto(APP_URL, { waitUntil: 'domcontentloaded' });

	// Wait UI ready (server config loaded renders content)
	await page.getByText('Character Configuration').waitFor({ timeout: 60000 });

	// Fill Base Model Wowhead URL
	const npcInput = page.getByPlaceholder(/wowhead\.com\/(?:[a-z-]+\/)?npc=/i).first();
	await npcInput.click();
	await npcInput.fill(NPC_URL);
	await npcInput.blur();

	// Fill attached item Wowhead URL (Item Reference)
	const itemInput = page.getByPlaceholder(/wowhead\.com\/(?:[a-z-]+\/)?item=/i).first();
	await itemInput.click();
	await itemInput.fill(ITEM_URL);
	await itemInput.blur();

	// Set output file name explicitly to ensure validation
	const filenameInput = page.locator('#filename');
	await filenameInput.scrollIntoViewIfNeeded();
	await filenameInput.fill('lady-deathwhisper');

	// Try to select Hand Right attachment point if present (best-effort)
	try {
		const attachmentLabel = page.getByText('Attachment Point');
		await attachmentLabel.first().scrollIntoViewIfNeeded();
		const combo = attachmentLabel.first().locator('..').locator('..').getByRole('combobox');
		await combo.first().click();
		await page.getByRole('option', { name: /Hand Right \(1\)/ }).click();
	} catch (e) {
		console.warn('Attachment point selection skipped or not needed:', e?.message || e);
	}

	// Click Export Character using robust text selector
	const exportBtn = page.locator('button:has-text("Export Character")');
	await exportBtn.scrollIntoViewIfNeeded();
	await exportBtn.waitFor({ state: 'visible', timeout: 60000 });
	await exportBtn.click();

	// Wait for completion: success text or model viewer canvas
	await Promise.race([
		page.getByText(/Export Successful/i).waitFor({ timeout: 600_000 }),
		page.locator('canvas').first().waitFor({ state: 'visible', timeout: 600_000 })
	]);

	// Scroll to model viewer and screenshot the canvas
	const canvas = page.locator('canvas').first();
	await canvas.scrollIntoViewIfNeeded();
	await canvas.screenshot({ path: SCREENSHOT_PATH });
	console.log('Saved screenshot to', SCREENSHOT_PATH);

	await browser.close();
}

run().catch((err) => {
	console.error(err);
	process.exit(1);
});