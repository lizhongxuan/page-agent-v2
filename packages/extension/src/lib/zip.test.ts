import { describe, expect, it } from 'vitest'

import { createZipBlob } from './zip'

describe('createZipBlob', () => {
	it('creates a zip archive with local, central, and end directory records', async () => {
		const blob = createZipBlob({
			'package.json': '{"scripts":{"test":"vitest run"}}',
			'tests/archive.spec.ts': 'test("archive", async () => {})',
		})
		const bytes = new Uint8Array(await blob.arrayBuffer())
		const view = new DataView(bytes.buffer)

		expect(blob.type).toBe('application/zip')
		expect(view.getUint32(0, true)).toBe(0x04034b50)
		expect(findSignature(bytes, [0x50, 0x4b, 0x01, 0x02])).toBeGreaterThan(0)
		expect(findSignature(bytes, [0x50, 0x4b, 0x05, 0x06])).toBeGreaterThan(0)
		expect(new TextDecoder().decode(bytes)).toContain('tests/archive.spec.ts')
	})
})

function findSignature(bytes: Uint8Array, signature: number[]) {
	for (let index = 0; index <= bytes.length - signature.length; index++) {
		if (signature.every((byte, offset) => bytes[index + offset] === byte)) return index
	}
	return -1
}
