const APP = process.env.APP_URL || 'http://127.0.0.1:3001';

const NPC_URL = 'https://www.wowhead.com/wotlk/npc=36855/lady-deathwhisper';
const ITEM_URL = 'https://www.wowhead.com/item=120978/ashbringer';

async function postExport() {
	const body = {
		character: {
			base: { type: 'wowhead', value: NPC_URL },
			attackTag: 'Auto',
			inGameMovespeed: 270,
			attachItems: {
				1: { path: { type: 'wowhead', value: ITEM_URL } },
			},
		},
		outputFileName: 'lady-deathwhisper',
		optimization: {},
		format: 'mdx',
		formatVersion: '1000',
	};
	const res = await fetch(`${APP}/export/character`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body),
	});
	if (!res.ok) throw new Error(await res.text());
	return res.json();
}

async function poll(jobId) {
	for (let i = 0; i < 1200; i++) { // up to 20 min
		const res = await fetch(`${APP}/export/character/status/${jobId}`);
		if (!res.ok) throw new Error(await res.text());
		const json = await res.json();
		if (json.status === 'done') return json;
		if (json.status === 'failed') throw new Error(json.error || 'failed');
		await new Promise((r) => setTimeout(r, 1000));
	}
	throw new Error('timeout');
}

(async () => {
	const queued = await postExport();
	const status = await poll(queued.id);
	console.log(JSON.stringify(status, null, 2));
})();