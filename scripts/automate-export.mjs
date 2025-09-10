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
	await page.goto(APP_URL, { waitUntil: 'load' });

	// Fill Base Model Wowhead URL
	const npcInput = page.getByPlaceholder(/wowhead\.com\/(?:[a-z-]+\/)?npc=/i);
	await npcInput.click();
	await npcInput.fill(NPC_URL);
	await npcInput.blur();

	// Fill attached item Wowhead URL (Item Reference)
	const itemInput = page.getByPlaceholder(/wowhead\.com\/(?:[a-z-]+\/)?item=/i);
	await itemInput.click();
	await itemInput.fill(ITEM_URL);
	await itemInput.blur();

	// Ensure Attachment Point is Hand Right (1) if selector exists
	try {
		// Open the closest combobox after the "Attachment Point" label
		const attachmentLabel = page.getByText('Attachment Point');
		await attachmentLabel.first().scrollIntoViewIfNeeded();
		const comboboxes = page.getByRole('combobox');
		await comboboxes.first().click();
		await page.getByRole('option', { name: /Hand Right \(1\)/ }).click();
	} catch (e) {
		console.warn('Attachment point selection skipped or not needed:', e?.message || e);
	}

	// Click Export Character
	await page.getByRole('button', { name: /Export Character/i }).click();

	// Wait for completion: either Export Successful text or model viewer canvas appears
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